package dao

import (
	"context"
	"time"

	"encore.app/fee/model"
)

type DB interface {
	CreateBill(ctx context.Context, billID, policyType string) error
	GetBillStatus(ctx context.Context, billID string) (model.BillStatus, error)
	InsertLineItem(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata) error
	UpdateBillTotal(ctx context.Context, billID, currency string, amount int64) error
	CloseBill(ctx context.Context, billID string, metadata model.BillMetadata) error
	GetBill(ctx context.Context, billID string) (*model.BillDetail, error)
	GetLineItemsForBill(ctx context.Context, billID string) ([]model.LineItem, error)
	GetBills(ctx context.Context, status model.BillStatus, limit int, cursor time.Time) ([]*model.BillDetail, bool, error)
	GetBillIDs(ctx context.Context, status model.BillStatus, policyType model.PolicyType, limit int, cursor time.Time) ([]string, bool, error)
	AddLineItem(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata, lineItemID string) error
	UpdateLineItem(ctx context.Context, billID, lineItemID, status string) (*model.LineItem, error)
	IsBillExists(ctx context.Context, billID string) (bool, error)
}

//go:generate mockery --name=DB --output=./mocks --outpkg=mocks
