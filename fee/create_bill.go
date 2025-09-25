package fee

import (
	"context"
	"errors"

	"encore.app/fee/dao"
	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"encore.dev/rlog"
	"go.temporal.io/sdk/client"
)

type CreateBillParams struct {
	BillID         string `json:"bill_id"`
	FeePolicyType  string `json:"fee_policy_type"`
	IdempotencyKey string `header:"X-Idempotency-Key"`
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
		return nil, err
	}
	// 1. Create bill entry in the database first
	err = dao.CreateBill(ctx, params.BillID, params.FeePolicyType)
	if err != nil {
		rlog.Error("failed to create bill in database", "error", err, "bill_id", params.BillID)
		return nil, err
	}

	// 2. Start Temporal workflow only if DB insertion is successful
	req := &temporal.BillLifecycleWorkflowRequest{
		BillID:     params.BillID,
		PolicyType: policyType,
	}

	_, err = s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "bill-" + params.BillID,
		TaskQueue: billCycleTaskQueue,
	}, temporal.BillLifecycleWorkflow, req)
	if err != nil {
		rlog.Error("failed to start bill lifecycle workflow after successful DB insert", "error", err, "bill_id", params.BillID)
		return nil, errors.New("bill created but workflow failed to start: " + err.Error())
	}

	return &CreateBillResponse{BillID: params.BillID, Status: string(model.BillStatusOpen)}, nil
}
