package fee

import (
	"context"
	"time"

	"encore.app/fee/dao"
	"encore.app/fee/model"
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
	var status model.BillStatus
	if params.Status != "" {
		var err error
		status, err = model.ToBillStatus(params.Status)
		if err != nil {
			rlog.Error("invalid status", "error", err)
			return nil, err
		}
	}

	if params.Limit == 0 {
		params.Limit = 10
	}

	var cursor time.Time
	if params.Cursor != "" {
		var err error
		cursor, err = time.Parse(time.RFC3339, params.Cursor)
		if err != nil {
			return nil, err
		}
	}

	bills, hasMore, err := dao.GetBills(ctx, status, params.Limit, cursor)
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
		resp.Bills[i].TotalCharges = make([]TotalSummary, len(bill.TotalCharges))
		for j, total := range bill.TotalCharges {
			resp.Bills[i].TotalCharges[j].Currency = total.Currency
			resp.Bills[i].TotalCharges[j].DisplayAmount = total.DisplayAmount
			resp.Bills[i].TotalCharges[j].TotalAmount = total.TotalAmount
		}
	}

	return resp, nil
}
