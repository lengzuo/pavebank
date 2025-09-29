package temporal

import (
	"time"

	"encore.app/fee/model"
	"encore.app/fee/utils"
)

type RecurringPolicy struct {
	Amount      int64
	Interval    utils.Duration
	Description string
}

type BillLifecycleWorkflowRequest struct {
	BillID            string
	PolicyType        model.PolicyType
	BilingPeriodStart time.Time
	BillingPeriodEnd  time.Time
	Currency          string
	Recurring         RecurringPolicy
	PreviousState     *BillState
}

type BillClosedPostProcessWorkflowRequest struct {
	BillID string
}
type AddLineItemSignalRequest struct {
	LineItemID string
	Amount     int64
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
	BillID        string     `json:"bill_id"`
	Status        string     `json:"status"`
	PolicyType    string     `json:"policy_type"`
	CreatedAt     time.Time  `json:"created_at"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`
	Currency      string     `json:"currency"`
	TotalAmount   int64      `json:"total_amount"`
	DisplayAmount string     `json:"display_amount"`
	LineItems     []LineItem `json:"line_items"`
}
