package fee

import (
	"context"
	"errors"
	"fmt"
	"time"

	"encore.app/fee/model"
	"encore.app/fee/utils"
	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"go.temporal.io/sdk/client"
)

type Recurring struct {
	Amount      int64          `json:"amount"`
	Interval    utils.Duration `json:"interval"`
	Description string         `json:"description"`
}

type CreateBillParams struct {
	BillID             string     `json:"bill_id"`
	PolicyType         string     `json:"policy_type"`
	Currency           string     `json:"currency"`
	Recurring          *Recurring `json:"recurring"`
	BillingPeriodStart time.Time  `json:"billing_period_start"`
	BillingPeriodEnd   time.Time  `json:"billing_period_end"` // eg: 2025-09-25T21:25:00+08:00
	IdempotencyKey     string     `header:"X-Idempotency-Key"`
}

func (p *CreateBillParams) Validate() error {
	if p.BillID == "" {
		return fmt.Errorf("bill_id is a required field")
	}
	_, err := model.ToCurrency(p.Currency)
	if err != nil {
		return err
	}
	if p.BillingPeriodEnd.IsZero() {
		return fmt.Errorf("billing_period_end is a required field")
	}
	if p.BillingPeriodStart.IsZero() {
		p.BillingPeriodStart = time.Now()
	}
	if p.BillingPeriodEnd.Before(time.Now().Add(1 * time.Minute)) {
		return fmt.Errorf("billing_period_end is too short, must be at least 1 minutes ahead")
	}
	policy, err := model.ToPolicyType(p.PolicyType)
	if err != nil {
		return fmt.Errorf("invalid policy")
	}

	if policy == model.Subscription {
		if p.Recurring == nil {
			return fmt.Errorf("recurring is mandatory for policy=SUBSCRIPTION")
		}
		if p.Recurring.Amount <= 0 {
			return fmt.Errorf("recurring.amount must be more than zero")
		}
		if p.Recurring.Interval.Duration <= 0 {
			return fmt.Errorf("recurring.interval must be at provided")
		}

	}
	return nil
}

type CreateBillResponse struct {
	BillID string `json:"bill_id"`
	Status string `json:"status"`
}

//encore:api public method=POST path=/bills tag:idempotency
func (s *Service) CreateBill(ctx context.Context, params *CreateBillParams) (*CreateBillResponse, error) {
	if err := params.Validate(); err != nil {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: err.Error(),
		}
	}

	exists, err := s.db.IsBillExists(ctx, params.BillID)
	if err != nil {
		rlog.Error("failed in checking bill exists in db", "error", err)
		return nil, err
	}
	if exists {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "duplicate bill id",
		}
	}

	req := &temporal.BillLifecycleWorkflowRequest{
		BillID:            params.BillID,
		PolicyType:        model.PolicyType(params.PolicyType),
		BillingPeriodEnd:  params.BillingPeriodEnd,
		BilingPeriodStart: params.BillingPeriodStart,
		Currency:          params.Currency,
	}
	if params.Recurring != nil {
		recurring := params.Recurring
		req.Recurring = temporal.RecurringPolicy{
			Amount:      recurring.Amount,
			Interval:    recurring.Interval,
			Description: recurring.Description,
		}
	}

	_, err = s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        temporal.BillCycleWorkflowID(params.BillID),
		TaskQueue: temporal.BillCycleTaskQueue,
	}, temporal.BillLifecycleWorkflow, req)
	if err != nil {
		rlog.Error("failed to start bill lifecycle workflow after successful DB insert", "error", err, "bill_id", params.BillID)
		return nil, errors.New("failed to start workflow: " + params.BillID)
	}

	return &CreateBillResponse{BillID: params.BillID, Status: string(model.BillStatusOpen)}, nil
}
