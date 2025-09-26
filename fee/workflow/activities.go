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
	err := dao.AddLineItemTx(ctx, billID, currency, amount, metadata)
	if err != nil {
		if errors.Is(err, dao.ErrBillNotFound) {
			return temporal.NewNonRetryableApplicationError(err.Error(), billNotFound, err)
		}
		if errors.Is(err, dao.ErrBillIsClosed) {
			return temporal.NewNonRetryableApplicationError(err.Error(), billClosed, err)
		}
		// For any other error, let Temporal's retry policy handle it.
		return fmt.Errorf("failed to add line item transactionally: %w", err)
	}
	return nil
}

func (a *Activities) CreateBill(ctx context.Context, billID string, policyType model.PolicyType) error {
	return dao.CreateBill(ctx, billID, string(policyType))
}

func (a *Activities) CloseBill(ctx context.Context, billID string) error {
	// Update the bill status
	err := dao.CloseBill(ctx, billID)
	// TODO: Generate PDF invoice using html2pdf
	err = a.GeneratePDFInvoive(ctx, billID)
	// TODO: Create payment link
	err = a.CreatePaymentLink(ctx, billID)
	// TODO: Email the PDF to client using sendgrid.
	err = a.SendBillEmail(ctx, billID)
	return err
}

func (a *Activities) GetBill(ctx context.Context, billID, userID string) (*model.Bill, error) {
	return dao.GetBill(ctx, billID)
}

func (a *Activities) GetBillSummary(ctx context.Context, billID string) (*model.BillSummary, error) {
	// First, get the basic bill details
	var billSummary model.BillSummary
	billHeader, err := dao.GetBill(ctx, billID)
	if err != nil {
		if errors.Is(err, errors.New("bill not found")) {
			return nil, temporal.NewNonRetryableApplicationError("bill not found", "BillNotFoundError", err)
		}
		return nil, fmt.Errorf("failed to get bill header: %w", err)
	}

	billSummary.BillID = billHeader.BillID
	billSummary.PolicyType = billHeader.PolicyType
	billSummary.Status = billHeader.Status
	billSummary.CreatedAt = billHeader.CreatedAt
	billSummary.ClosedAt = billHeader.ClosedAt

	// Get bill totals
	totalCharges, err := dao.GetBillTotalsForBill(ctx, billID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bill totals: %w", err)
	}
	billSummary.TotalCharges = totalCharges

	return &billSummary, nil
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
