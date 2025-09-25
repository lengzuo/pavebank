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
	Amount         int64                   `json:"amount"`
	Currency       string                  `json:"currency"`
	Metadata       *model.LineItemMetadata `json:"metadata,omitempty"`
	IdempotencyKey string                  `header:"X-Idempotency-Key"`
}

//encore:api public method=POST path=/bills/:billID/line-items tag:idempotency
func (s *Service) AddLineItem(ctx context.Context, billID string, params *AddLineItemParams) error {
	if params.Amount <= 0 {
		return &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "amount must be positive",
		}
	}
	currency, err := model.ToCurrency(params.Currency)
	if err != nil {
		return &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "invalid currency",
		}
	}
	signal := temporal.AddLineItemSignalRequest{
		Amount:   params.Amount,
		Currency: currency,
		BillID:   billID,
		Metadata: params.Metadata,
	}

	err = s.client.SignalWorkflow(ctx, "bill-"+billID, "", temporal.AddLineItemSignal, signal)
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) || errors.Is(err, sql.ErrNoRows) {
			return &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found or already closed",
			}
		}
		rlog.Error("failed to signal add line item workflow", "error", err)
		return err
	}

	return nil
}
