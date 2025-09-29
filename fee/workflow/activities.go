package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"encore.app/fee/dao"
	"encore.app/fee/model"
	"go.temporal.io/sdk/temporal"
)

const (
	errNotFound = "NotFound"
)

type Activities struct {
	db dao.DB
}

func NewActivity(db dao.DB) *Activities {
	return &Activities{db: db}
}

func (a *Activities) AddLineItem(ctx context.Context, billID string, amount int64, metadata *model.LineItemMetadata, lineItemID string) error {
	err := a.db.AddLineItem(ctx, billID, amount, metadata, lineItemID)
	if err != nil {
		return fmt.Errorf("failed to add line item: %s", err)
	}
	return nil
}

func (a *Activities) UpdateLineItem(ctx context.Context, billID, lineItemID, status string) (*model.LineItem, error) {
	lineItem, err := a.db.UpdateLineItem(ctx, billID, lineItemID, status)
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError("failed in update line item", errNotFound, err)
	}
	return lineItem, nil
}

func (a *Activities) CreateBill(ctx context.Context, billID, policyType, currency string, startAt time.Time, recurring RecurringPolicy) error {
	metadata := model.BillMetadata{}
	if recurring.Amount > 0 && recurring.Interval.Duration > 0 {
		metadata.Recurring = &model.Recurring{
			Description: recurring.Description,
			Amount:      recurring.Amount,
			Interval:    recurring.Interval.String(),
		}
	}

	return a.db.CreateBill(ctx, billID, string(policyType), currency, startAt, metadata)
}

func (a *Activities) CloseBillFromState(ctx context.Context, state BillState) error {
	err := a.db.CloseBill(ctx, state.BillID, state.Total)
	return err
}

func (a *Activities) GetBillDetail(ctx context.Context, billID string) (*BillResponse, error) {
	bill, err := a.db.GetBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	resp := &BillResponse{
		BillID:        bill.BillID,
		Currency:      bill.Currency,
		Status:        bill.Status,
		PolicyType:    bill.PolicyType,
		CreatedAt:     bill.CreatedAt,
		ClosedAt:      bill.ClosedAt,
		TotalAmount:   bill.TotalAmount,
		DisplayAmount: model.FormatAmount(bill.TotalAmount),
	}

	resp.LineItems = make([]LineItem, 0, len(bill.LineItems))
	for _, item := range bill.LineItems {
		var metadata model.LineItemMetadata
		err := json.Unmarshal([]byte(item.Metadata), &metadata)
		if err != nil {
			return nil, err
		}
		resp.LineItems = append(resp.LineItems, LineItem{
			LineItemID:    item.LineItemID,
			Currency:      bill.Currency,
			Amount:        item.Amount,
			Description:   metadata.Description,
			CreatedAt:     item.CreatedAt,
			DisplayAmount: model.FormatAmount(item.Amount),
			Status:        item.Status,
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
