package model

import (
	"fmt"
	"strings" // Import for string manipulation
	"time"

	"github.com/shopspring/decimal"
)

// Currency represents the currency of a bill.
type Currency string

const (
	USD Currency = "USD"
	GEL Currency = "GEL"
)

// BillStatus represents the status of a bill.
type BillStatus string

const (
	BillStatusOpen    BillStatus = "OPEN"
	BillStatusClosed  BillStatus = "CLOSED"
	BillStatusSettled BillStatus = "SETTLED"
)

func ToBillStatus(s string) (BillStatus, error) {
	switch BillStatus(s) {
	case BillStatusOpen:
		return BillStatusOpen, nil
	case BillStatusClosed:
		return BillStatusClosed, nil
	case BillStatusSettled:
		return BillStatusSettled, nil
	default:
		return "", fmt.Errorf("invalid BillStatus: %s", s)
	}
}

// LineItemStatus represents the status of a bill.
type LineItemStatus string

const (
	LineItemStatusActive LineItemStatus = "ACTIVE"
	LineItemStatusVoided LineItemStatus = "VOIDED"
)

// ToCurrency converts a string to a Currency type, validating it against known currencies.
// It accepts lowercase inputs and converts them to uppercase for validation.
func ToCurrency(s string) (Currency, error) {
	upperS := strings.ToUpper(s)
	switch Currency(upperS) {
	case USD:
		return USD, nil
	case GEL:
		return GEL, nil
	default:
		return "", fmt.Errorf("invalid Currency: %s", s)
	}
}

// PolicyType represents the bill policy type.
type PolicyType string

const (
	PolicyTypeUsageBased PolicyType = "USAGE_BASED"
	PolicyTypeMonthly    PolicyType = "MONTHLY"
)

func ToPolicyType(s string) (PolicyType, error) {
	switch PolicyType(s) {
	case PolicyTypeUsageBased:
		return PolicyTypeUsageBased, nil
	case PolicyTypeMonthly:
		return PolicyTypeMonthly, nil
	default:
		return "", fmt.Errorf("invalid PolicyType: %s", s)
	}
}

// FormatAmount converts an int64 amount (in cents) to a string with two decimal places.
func FormatAmount(amount int64) string {
	return decimal.NewFromInt(amount).Div(decimal.NewFromInt(100)).StringFixed(2)
}

type BillMetadata struct {
	TotalAmounts         map[string]int64 `json:"total_amounts"`
	FinalChargedAmount   int64            `json:"final_charged_amount"`
	FinalChargedCurrency string           `json:"final_charged_currency"`
}

type Bill struct {
	BillID     string     `json:"bill_id"`
	PolicyType string     `json:"policy_type"`
	Status     string     `json:"status"`
	Metadata   string     `json:"metadata"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ClosedAt   *time.Time `json:"closed_at"`
}

type LineItemMetadata struct {
	Description string `json:"description"`
}

type LineItem struct {
	LineItemID string    `json:"line_item_id"`
	Metadata   string    `json:"metadata"`
	Currency   string    `json:"currency"`
	Amount     int64     `json:"amount"`
	CreatedAt  time.Time `json:"created_at"`
	Status     string    `json:"status"`
}

type LineItemSummary struct {
	Currency string `json:"currency"`
	Amount   int64  `json:"amount"`
}

type TotalSummary struct {
	Currency    string `json:"currency"`
	TotalAmount int64  `json:"total_amount"`
}

type BillDetail struct {
	BillID       string           `json:"bill_id"`
	Status       string           `json:"status"`
	PolicyType   string           `json:"policy_type"`
	CreatedAt    time.Time        `json:"created_at"`
	ClosedAt     *time.Time       `json:"closed_at,omitempty"`
	LineItems    []LineItem       `json:"line_items"`
	TotalCharges map[string]int64 `json:"total_charges"`
}
