package temporal

import (
	"context"
	"errors"
	"fmt"

	"encore.app/fee/dao"
	"encore.app/fee/model"
	"go.temporal.io/sdk/temporal"
)

const (
	billClosed   = "BillClosedError"
	billNotFound = "BillNotFoundError"
)

type Activities struct{}

func (a *Activities) AddLineItem(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata) error {
	err := dao.AddLineItem(ctx, billID, currency, amount, metadata)
	if err != nil {
		return fmt.Errorf("failed to add line item: %s", err)
	}
	return nil
}

func (a *Activities) CreateBill(ctx context.Context, billID string, policyType model.PolicyType) error {
	return dao.CreateBill(ctx, billID, string(policyType))
}

// func (a *Activities) CloseBill(ctx context.Context, billID string) error {
// 	// Update the bill status
// 	err := dao.CloseBill(ctx, billID)
// 	// TODO: Generate PDF invoice using html2pdf
// 	err = a.GeneratePDFInvoive(ctx, billID)
// 	// TODO: Create payment link
// 	err = a.CreatePaymentLink(ctx, billID)
// 	// TODO: Email the PDF to client using sendgrid.
// 	err = a.SendBillEmail(ctx, billID)
// 	return err
// }

func (a *Activities) CloseBillFromState(ctx context.Context, state BillState) error {
	billMetadata := model.BillMetadata{
		TotalAmounts: state.Totals,
	}
	err := dao.CloseBill(ctx, state.BillID, billMetadata)
	return err
}

func (a *Activities) GetBillDetail(ctx context.Context, billID string) (*model.BillDetail, error) {
	var billDetail *model.BillDetail
	billDetail, err := dao.GetBill(ctx, billID)
	if err != nil {
		if errors.Is(err, errors.New("bill not found")) {
			return nil, temporal.NewNonRetryableApplicationError("bill not found", billNotFound, err)
		}
		return nil, fmt.Errorf("failed to get bill header: %w", err)
	}
	return billDetail, nil
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
