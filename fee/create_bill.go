package fee

import (
	"context"
	"errors"
	"time"

	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"go.temporal.io/sdk/client"
)

type CreateBillParams struct {
	BillID           string    `json:"bill_id"`
	FeePolicyType    string    `json:"fee_policy_type"`
	BillingPeriodEnd time.Time `json:"billing_period_end"` // eg: 2025-09-25T21:25:00+08:00
	IdempotencyKey   string    `header:"X-Idempotency-Key"`
}

type CreateBillResponse struct {
	BillID string `json:"bill_id"`
	Status string `json:"status"`
}

//encore:api public method=POST path=/bills tag:idempotency
func (s *Service) CreateBill(ctx context.Context, params *CreateBillParams) (*CreateBillResponse, error) {
	policyType, err := model.ToPolicyType(params.FeePolicyType)
	if err != nil {
		rlog.Error("failed to create bill policy type is mandatory", "error", err, "bill_id", params.BillID)
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: err.Error(),
		}
	}

	if params.BillingPeriodEnd.IsZero() {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "billing_period_end is a required field",
		}
	}

	req := &temporal.BillLifecycleWorkflowRequest{
		BillID:           params.BillID,
		PolicyType:       policyType,
		BillingPeriodEnd: params.BillingPeriodEnd,
	}

	_, err = s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "bill-" + params.BillID,
		TaskQueue: billCycleTaskQueue,
	}, temporal.BillLifecycleWorkflow, req)
	if err != nil {
		rlog.Error("failed to start bill lifecycle workflow after successful DB insert", "error", err, "bill_id", params.BillID)
		return nil, errors.New("failed to start workflow: " + params.BillID)
	}

	return &CreateBillResponse{BillID: params.BillID, Status: string(model.BillStatusOpen)}, nil
}
