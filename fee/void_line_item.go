package fee

import (
	"context"
	"database/sql"
	"errors"

	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
	"go.temporal.io/api/serviceerror"
)

type RemoveLineItemResponse struct {
	LineItemID string `json:"line_item_id"`
	BillID     string `json:"bill_id"`
}

//encore:api public method=PUT path=/bills/:billID/line-items/:lineItemID/void tag:idempotency
func (s *Service) VoidLineItem(ctx context.Context, billID, lineItemID string) (*RemoveLineItemResponse, error) {
	signal := temporal.UpdateLineItemSignalRequest{
		LineItemID: lineItemID,
		BillID:     billID,
		Status:     model.LineItemStatusVoided,
	}
	workflowID := temporal.BillCycleWorkflowID(billID)

	err := s.client.SignalWorkflow(ctx, workflowID, "", temporal.UpdateLineItemSignal, signal)
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found or already closed",
			}
		}
		rlog.Error("failed to signal update line item workflow", "error", err)
		return nil, err
	}

	return &RemoveLineItemResponse{
		LineItemID: lineItemID,
		BillID:     billID,
	}, nil
}
