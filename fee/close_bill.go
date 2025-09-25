package fee

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
)

type CloseBillParams struct {
	IdempotencyKey string `header:"X-Idempotency-Key"`
}

//encore:api public method=POST path=/bills/:billID/close tag:idempotency
func (s *Service) CloseBill(ctx context.Context, billID string, params *CloseBillParams) (*model.BillSummary, error) {
	if billID == "" {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "billId is mandatory",
		}
	}

	workflowID := "bill-" + billID
	signal := temporal.ClosedBillRequest{
		BillID: billID,
	}

	// Signal the workflow to close the bill
	err := s.client.SignalWorkflow(ctx, workflowID, "", temporal.CloseBillSignal, signal)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found or already closed",
			}
		}
		rlog.Error("failed to signal workflow to close bill", "error", err, "bill_id", billID)
		return nil, fmt.Errorf("failed to signal workflow: %w", err)
	}

	// Get a handle to the workflow run
	run := s.client.GetWorkflow(ctx, workflowID, "")

	// Wait for the workflow to complete and retrieve the result
	var billSummary model.BillSummary
	err = run.Get(ctx, &billSummary)
	if err != nil {
		rlog.Error("failed to get workflow result", "error", err, "bill_id", billID)
		return nil, fmt.Errorf("failed to get workflow result: %w", err)
	}

	return &billSummary, nil
}
