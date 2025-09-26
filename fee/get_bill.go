package fee

import (
	"context"
	"database/sql"
	"errors"

	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
)

//encore:api public method=GET path=/bills/:billID
func (s *Service) GetBill(ctx context.Context, billID string) (*temporal.BillResponse, error) {
	resp, err := s.activity.GetBillDetail(ctx, billID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &errs.Error{
				Code:    errs.NotFound,
				Message: "bill not found or already closed",
			}
		}
		rlog.Error("failed to get bill", "error", err)
		return nil, err
	}
	return resp, nil
}
