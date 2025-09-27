package fee

import (
	"context"
	"strings"
	"time"

	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"encore.dev/rlog"
)

type TotalSummary struct {
	Currency      string `json:"currency"`
	TotalAmount   int64  `json:"total_amount"`
	DisplayAmount string `json:"display_amount"`
}

type Bill struct {
	BillID       string         `json:"bill_id"`
	Status       string         `json:"status"`
	PolicyType   string         `json:"policy_type"`
	CreatedAt    time.Time      `json:"created_at"`
	ClosedAt     *time.Time     `json:"closed_at,omitempty"`
	TotalCharges []TotalSummary `json:"total_charges"`
}

type GetBillsResponse struct {
	Bills   []Bill `json:"bills"`
	HasMore bool   `json:"has_more"`
}

type GetBillsParams struct {
	Status string `query:"status"`
	Limit  int    `query:"limit"`
	Cursor string `query:"cursor"`
}

//encore:api public method=GET path=/bills
func (s *Service) GetBills(ctx context.Context, params *GetBillsParams) (*GetBillsResponse, error) {
	status, err := model.ToBillStatus(strings.ToUpper(params.Status))
	if err != nil {
		rlog.Error("invalid status", params.Status, "error", err)
		return nil, &errs.Error{
			Code:    errs.InvalidArgument,
			Message: "invalid status",
		}
	}

	if params.Limit == 0 {
		params.Limit = 10
	}

	cursor := time.Now()
	if params.Cursor != "" {
		var err error
		cursor, err = time.Parse(time.RFC3339, params.Cursor)
		if err != nil {
			return nil, err
		}
	}

	bills, hasMore, err := s.db.GetBills(ctx, status, params.Limit, cursor)
	if err != nil {
		rlog.Error("failed to get bills", "error", err)
		return nil, err
	}
	resp := &GetBillsResponse{
		HasMore: hasMore,
	}
	resp.Bills = make([]Bill, len(bills))
	for i, bill := range bills {
		resp.Bills[i] = Bill{
			BillID:     bill.BillID,
			Status:     string(bill.Status),
			PolicyType: bill.PolicyType,
			CreatedAt:  bill.CreatedAt,
			ClosedAt:   bill.ClosedAt,
		}
		// Query from temporal state if bills is still open
		if bill.Status == string(model.BillStatusOpen) {
			workflowID := "bill-" + bill.BillID
			queryResult, err := s.client.QueryWorkflow(ctx, workflowID, "", temporal.QueryBillTotal)
			if err != nil {
				rlog.Error("failed to query workflow for live totals", "error", err, "bill_id", bill.BillID)
				resp.Bills[i].TotalCharges = []TotalSummary{}
				continue
			}
			var totals map[string]int64
			if err := queryResult.Get(&totals); err != nil {
				rlog.Error("failed to decode workflow query bill total", "error", err, "bill_id", bill.BillID)
				continue
			}
			resp.Bills[i].TotalCharges = make([]TotalSummary, 0, len(totals))
			for currency, amount := range totals {
				resp.Bills[i].TotalCharges = append(resp.Bills[i].TotalCharges, TotalSummary{
					Currency:      currency,
					TotalAmount:   amount,
					DisplayAmount: model.FormatAmount(amount),
				})
			}
			continue
		}
		resp.Bills[i].TotalCharges = make([]TotalSummary, 0, len(bill.TotalCharges))
		for currency := range bill.TotalCharges {
			amount := bill.TotalCharges[currency]
			resp.Bills[i].TotalCharges = append(resp.Bills[i].TotalCharges, TotalSummary{
				Currency:      currency,
				TotalAmount:   amount,
				DisplayAmount: model.FormatAmount(amount),
			})
		}
	}

	return resp, nil
}
