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

type AddLineItemParams struct {
	Amount         int64  `json:"amount"`
	Currency       string `json:"currency"`
	Description    string `json:"description"`
	IdempotencyKey string `header:"X-Idempotency-Key"`
}

type AddLineItemResponse struct {
	Amount      int64  `json:"amount"`
	Currency    string `json:"currency"`
	BillID      string `json:"bill_id"`
	Description string `json:"description"`
}

//encore:api public method=POST path=/bills/:billID/line-items tag:idempotency
func (s *Service) AddLineItem(ctx context.Context, billID string, params *AddLineItemParams) (*AddLineItemResponse, error) {
	if params.Amount <= 0 {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "amount must be positive",
		}
	}
	currency, err := model.ToCurrency(params.Currency)
	if err != nil {
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "invalid currency",
		}
	}
	signal := temporal.AddLineItemSignalRequest{
		Amount:   params.Amount,
		Currency: currency,
		BillID:   billID,
	}
	if params.Description != "" {
		signal.Metadata = &model.LineItemMetadata{Description: params.Description}
	}
	workflowID := temporal.BillCycleWorkflowID(billID)

	err = s.client.SignalWorkflow(ctx, workflowID, "", temporal.AddLineItemSignal, signal)
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found or already closed",
			}
		}
		rlog.Error("failed to signal add line item workflow", "error", err)
		return nil, err
	}

	return &AddLineItemResponse{
		Amount:      params.Amount,
		Currency:    params.Currency,
		BillID:      billID,
		Description: params.Description,
	}, nil
}
