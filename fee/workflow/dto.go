package temporal

import (
	"time"

	"encore.app/fee/model"
)

type BillLifecycleWorkflowRequest struct {
	BillID           string
	PolicyType       model.PolicyType
	BillingPeriodEnd time.Time
	PreviousState    *BillState
	SubscriptionFee  *model.LineItemSummary
}

type BillClosedPostProcessWorkflowRequest struct {
	BillID string
}
type AddLineItemSignalRequest struct {
	LineItemID string
	Amount     int64
	Currency   model.Currency
	BillID     string
	Metadata   *model.LineItemMetadata
}

type UpdateLineItemSignalRequest struct {
	LineItemID string
	BillID     string
	Status     model.LineItemStatus
}

type ClosedBillRequest struct {
	BillID string
}

type TotalSummary struct {
	Currency      string `json:"currency"`
	TotalAmount   int64  `json:"total_amount"`
	DisplayAmount string `json:"display_amount"`
}

type LineItem struct {
	LineItemID    string    `json:"line_item_id"`
	Currency      string    `json:"currency"`
	Amount        int64     `json:"amount"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	DisplayAmount string    `json:"display_amount"`
	Status        string    `json:"status"`
}

type BillResponse struct {
	BillID       string         `json:"bill_id"`
	Status       string         `json:"status"`
	PolicyType   string         `json:"policy_type"`
	CreatedAt    time.Time      `json:"created_at"`
	ClosedAt     *time.Time     `json:"closed_at,omitempty"`
	TotalCharges []TotalSummary `json:"total_charges"`
	LineItems    []LineItem     `json:"line_items"`
}
