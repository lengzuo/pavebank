package fee

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

type CloseBillParams struct {
	IdempotencyKey string `header:"X-Idempotency-Key"`
}

//encore:api public method=POST path=/api/bills/:billID/close tag:idempotency
func (s *Service) CloseBill(ctx context.Context, billID string, params *CloseBillParams) (*temporal.BillResponse, error) {
	if billID == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "billId is mandatory",
		}
	}

	signal := temporal.ClosedBillRequest{
		BillID: billID,
	}

	workflowID := temporal.BillCycleWorkflowID(billID)
	// Signal the workflow to close the bill
	err := s.client.SignalWorkflow(ctx, workflowID, "", temporal.CloseBillSignal, signal)
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found or already closed",
			}
		}
		rlog.Error("failed to signal workflow to close bill", "error", err, "bill_id", billID)
		return nil, fmt.Errorf("failed to signal workflow: %w", err)
	}

	// Get a handle to the workflow run
	we := s.client.GetWorkflow(ctx, workflowID, "")

	// Wait for the workflow to complete and retrieve the result
	var billDetail temporal.BillResponse
	err = we.GetWithOptions(ctx, &billDetail, client.WorkflowRunGetOptions{
		DisableFollowingRuns: false,
	})
	if err != nil {
		rlog.Error("failed to get workflow result", "error", err, "bill_id", billID)
		return nil, fmt.Errorf("failed to get workflow result: %w", err)
	}

	return &billDetail, nil
}
