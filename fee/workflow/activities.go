package temporal

import (
	"context"
	"encoding/json"
	"fmt"

	"encore.app/fee/dao"
	"encore.app/fee/model"
)

const (
	billClosed   = "BillClosedError"
	billNotFound = "BillNotFoundError"
)

type Activities struct {
	db dao.DB
}

func NewActivity(db dao.DB) *Activities {
	return &Activities{db: db}
}

func (a *Activities) AddLineItem(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata, uid string) error {
	err := a.db.AddLineItem(ctx, billID, currency, amount, metadata, uid)
	if err != nil {
		return fmt.Errorf("failed to add line item: %s", err)
	}
	return nil
}

func (a *Activities) CreateBill(ctx context.Context, billID string, policyType model.PolicyType) error {
	return a.db.CreateBill(ctx, billID, string(policyType))
}

func (a *Activities) CloseBillFromState(ctx context.Context, state BillState) error {
	billMetadata := model.BillMetadata{
		TotalAmounts: state.Totals,
	}
	err := a.db.CloseBill(ctx, state.BillID, billMetadata)
	return err
}

func (a *Activities) GetBillDetail(ctx context.Context, billID string) (*BillResponse, error) {
	bill, err := a.db.GetBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	resp := &BillResponse{
		BillID:     bill.BillID,
		Status:     bill.Status,
		PolicyType: bill.PolicyType,
		CreatedAt:  bill.CreatedAt,
		ClosedAt:   bill.ClosedAt,
	}
	resp.TotalCharges = make([]TotalSummary, 0, len(bill.TotalCharges))
	for currency, amount := range bill.TotalCharges {
		resp.TotalCharges = append(resp.TotalCharges, TotalSummary{
			Currency:      currency,
			TotalAmount:   amount,
			DisplayAmount: model.FormatAmount(amount),
		})
	}
	resp.LineItems = make([]LineItem, 0, len(bill.LineItems))
	for _, item := range bill.LineItems {
		var metadata model.LineItemMetadata
		err := json.Unmarshal([]byte(item.Metadata), &metadata)
		if err != nil {
			return nil, err
		}
		resp.LineItems = append(resp.LineItems, LineItem{
			Currency:      item.Currency,
			Amount:        item.Amount,
			Description:   metadata.Description,
			CreatedAt:     item.CreatedAt,
			DisplayAmount: model.FormatAmount(item.Amount),
		})
	}

	return resp, nil
}

func (a *Activities) GeneratePDFInvoive(ctx context.Context, billID string) error {
	// TODO: Implement generate PDF invoice
	return nil
}

func (a *Activities) SendBillEmail(ctx context.Context, billID string) error {
	// TODO: Implement email sending logic
	return nil
}

// CurrencyConversion used to convert USD → GEL
func (a *Activities) CurrencyConversion(ctx context.Context, billID string, targetCurrency string) (int64, error) {
	// TODO: Implement payment link creation logic
	return 0, nil
}

func (a *Activities) CreatePaymentLink(ctx context.Context, billID string) error {
	// TODO: Implement payment link creation logic
	return nil
}

func ComposeGreeting(ctx context.Context, name string) (string, error) {
	greeting := fmt.Sprintf("Hello123`123123123123123 %s!", name)
	return greeting, nil
}
