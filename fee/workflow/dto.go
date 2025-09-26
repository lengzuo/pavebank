package temporal

import (
	"time"

	"encore.app/fee/model"
)

type BillLifecycleWorkflowRequest struct {
	BillID           string
	PolicyType       model.PolicyType
	BillingPeriodEnd time.Time
}

type BillClosedPostProcessWorkflowRequest struct {
	BillID string
}

type AddLineItemSignalRequest struct {
	Amount   int64
	Currency model.Currency
	BillID   string
	Metadata *model.LineItemMetadata
}

type ClosedBillRequest struct {
	BillID string
}
