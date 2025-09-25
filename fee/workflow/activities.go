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
	status, err := dao.GetBillStatus(ctx, billID)
	if err != nil {
		if errors.Is(err, errors.New("bill not found")) {
			return temporal.NewNonRetryableApplicationError("bill not found", billNotFound, err)
		}
		return fmt.Errorf("failed to get bill status: %w", err)
	}

	if status == model.BillStatusClosed {
		return temporal.NewNonRetryableApplicationError("bill is closed", billClosed, nil)
	}

	if err := dao.InsertLineItem(ctx, billID, currency, amount, metadata); err != nil {
		return fmt.Errorf("failed to insert line item: %w", err)
	}

	if err := dao.UpdateBillTotal(ctx, billID, currency, amount); err != nil {
		return fmt.Errorf("failed to update bill total: %w", err)
	}

	return nil
}

func (a *Activities) CloseBill(ctx context.Context, billID string) error {
	// Update the bill status
	err := dao.CloseBill(ctx, billID)
	// TODO: Generate PDF invoice using html2pdf
	// TODO: Email the PDF to client using sendgrid.
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

func ComposeGreeting(ctx context.Context, name string) (string, error) {
	greeting := fmt.Sprintf("Hello123`123123123123123 %s!", name)
	return greeting, nil
}
