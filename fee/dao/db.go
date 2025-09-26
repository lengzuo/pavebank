package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"encoding/json" // Import for JSON unmarshaling
	"fmt"           // Import for error formatting

	"encore.app/fee/model"
	"encore.dev/rlog"
	"encore.dev/storage/sqldb"
)

var (
	ErrBillNotFound = errors.New("bill not found")
	ErrBillIsClosed = errors.New("bill is closed")
)

var db = sqldb.NewDatabase("fee", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

// CreateBill inserts a new bill into the database.
func CreateBill(ctx context.Context, billID, policyType string) error {
	_, err := db.Exec(ctx, `
		INSERT INTO bills (bill_id, policy_type, status, updated_at)
		VALUES ($1, $2, $3, now())
	`, billID, policyType, model.BillStatusOpen)
	if err != nil {
		return err
	}
	return nil
}

// GetBillStatus retrieves the status of a bill.
func GetBillStatus(ctx context.Context, billID string) (model.BillStatus, error) {
	var status model.BillStatus
	err := db.QueryRow(ctx, "SELECT status FROM bills WHERE bill_id = $1", billID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("bill not found")
		}
		return "", err
	}
	return status, nil
}

// InsertLineItem inserts a new line item for a bill.
func InsertLineItem(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal line item metadata: %w", err)
	}

	_, err = db.Exec(ctx, `
		INSERT INTO line_items (bill_id, currency, amount, metadata)
		VALUES ($1, $2, $3, $4)
	`, billID, currency, amount, metadataBytes)
	if err != nil {
		return err
	}
	return nil
}

// UpdateBillTotal updates the total amount for a bill in a specific currency within the metadata JSONB.
func UpdateBillTotal(ctx context.Context, billID, currency string, amount int64) error {
	// Retrieve current metadata
	var currentMetadataJSON []byte
	err := db.QueryRow(ctx, `SELECT metadata FROM bills WHERE bill_id = $1`, billID).Scan(&currentMetadataJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("bill not found")
		}
		return fmt.Errorf("failed to retrieve current metadata: %w", err)
	}

	var billMetadata model.BillMetadata
	if len(currentMetadataJSON) > 0 {
		if err := json.Unmarshal(currentMetadataJSON, &billMetadata); err != nil {
			return fmt.Errorf("failed to unmarshal current metadata: %w", err)
		}
	}

	if billMetadata.TotalAmounts == nil {
		billMetadata.TotalAmounts = make(map[string]int64)
	}
	billMetadata.TotalAmounts[currency] += amount

	// Marshal updated metadata back to JSON
	updatedMetadataJSON, err := json.Marshal(billMetadata)
	if err != nil {
		return fmt.Errorf("failed to marshal updated metadata: %w", err)
	}

	// Update the database with the new JSON
	_, err = db.Exec(ctx, `
		UPDATE bills
		SET metadata = $2
		WHERE bill_id = $1
	`, billID, updatedMetadataJSON)
	if err != nil {
		return fmt.Errorf("failed to update bill metadata: %w", err)
	}

	return nil
}

// CloseBill updates the status of a bill to closed.
func CloseBill(ctx context.Context, billID string) error {
	_, err := db.Exec(ctx, `
		UPDATE bills
		SET status = $1, closed_at = now()
		WHERE bill_id = $2
	`, model.BillStatusClosed, billID)
	if err != nil {
		return err
	}
	rlog.Debug("CloseBill success", "billID", billID)
	return nil
}

// GetBill retrieves a bill's main details.
func GetBill(ctx context.Context, billID string) (*model.Bill, error) {
	var bill model.Bill
	err := db.QueryRow(ctx, `
		SELECT status, policy_type, closed_at, created_at, updated_at
		FROM bills
		WHERE bill_id = $1
	`, billID).Scan(&bill.Status, &bill.PolicyType, &bill.ClosedAt, &bill.CreatedAt, &bill.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("bill not found")
		}
		return nil, err
	}
	bill.BillID = billID

	return &bill, nil
}

// GetLineItemsForBill retrieves all line items for a given bill.
func GetLineItemsForBill(ctx context.Context, billID string) ([]model.LineItemSummary, error) {
	rows, err := db.Query(ctx, `
		SELECT currency, amount
		FROM line_items
		WHERE bill_id = $1
		ORDER BY created_at DESC
	`, billID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lineItems []model.LineItemSummary
	for rows.Next() {
		var item model.LineItemSummary
		if err := rows.Scan(&item.Currency, &item.Amount); err != nil {
			return nil, err
		}
		item.DisplayAmount = model.FormatAmount(item.Amount)
		lineItems = append(lineItems, item)
	}
	return lineItems, nil
}

// GetBillTotalsForBill retrieves the total amounts for a bill from the metadata JSONB.
func GetBillTotalsForBill(ctx context.Context, billID string) ([]model.TotalSummary, error) {
	var jsonData []byte
	err := db.QueryRow(ctx, `SELECT metadata FROM bills WHERE bill_id = $1`, billID).Scan(&jsonData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("bill not found")
		}
		return nil, err
	}

	return calculateTotalCharges(jsonData)
}

func calculateTotalCharges(jsonData []byte) ([]model.TotalSummary, error) {
	var billMetadata model.BillMetadata
	if err := json.Unmarshal(jsonData, &billMetadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	var totalCharges []model.TotalSummary
	for currency, amount := range billMetadata.TotalAmounts {
		totalCharges = append(totalCharges, model.TotalSummary{
			Currency:      currency,
			TotalAmount:   amount,
			DisplayAmount: model.FormatAmount(amount),
		})
	}
	return totalCharges, nil
}

// GetBills retrieves a list of bills filtered by status.
func GetBills(ctx context.Context, status model.BillStatus, limit int, cursor time.Time) ([]*model.BillSummary, bool, error) {
	query := `
		SELECT bill_id, status, policy_type, created_at, metadata, closed_at
		FROM bills
		WHERE created_at < $1 AND status = $2 
		ORDER BY created_at DESC LIMIT $3
	`
	args := []interface{}{cursor, status, limit + 1}
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var bills []*model.BillSummary
	for rows.Next() {
		var bill model.BillSummary
		var jsonData []byte
		if err := rows.Scan(&bill.BillID, &bill.Status, &bill.PolicyType, &bill.CreatedAt, &jsonData, &bill.ClosedAt); err != nil {
			return nil, false, err
		}
		totalBillCharges, err := calculateTotalCharges(jsonData)
		if err != nil {
			return nil, false, err
		}
		bill.TotalCharges = totalBillCharges
		bills = append(bills, &bill)
	}

	hasMore := len(bills) > limit
	if hasMore {
		bills = bills[:limit]
	}

	return bills, hasMore, nil
}

// GetBillIDs retrieves a bill id from status policy type
func GetBillIDs(ctx context.Context, status model.BillStatus, policyType model.PolicyType, limit int, cursor time.Time) ([]string, bool, error) {
	query := `
		SELECT bill_id
		FROM bills
		WHERE created_at < $1 AND status = $2 AND policy_type = $3
		ORDER BY created_at DESC LIMIT $4
	`
	args := []interface{}{cursor, status, policyType, limit + 1}
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var billIDs []string
	for rows.Next() {
		var billID string
		if err := rows.Scan(&billID); err != nil {
			return nil, false, err
		}
		billIDs = append(billIDs, billID)
	}

	hasMore := len(billIDs) > limit
	if hasMore {
		billIDs = billIDs[:limit]
	}

	return billIDs, hasMore, nil
}

// AddLineItemTx adds a line item within a single database transaction to ensure atomicity.
func AddLineItemTx(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata) (err error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			rlog.Error("failed to rollback")
		}
	}()

	// 1. Get bill status and metadata in a single query, and lock the row for update.
	var status model.BillStatus
	var currentMetadataJSON []byte
	err = tx.QueryRow(ctx, "SELECT status, metadata FROM bills WHERE bill_id = $1 FOR UPDATE", billID).Scan(&status, &currentMetadataJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrBillNotFound
		}
		return fmt.Errorf("failed to get bill status: %w", err)
	}

	if status == model.BillStatusClosed {
		return ErrBillIsClosed
	}

	// 2. Insert the line item
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal line item metadata: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO line_items (bill_id, currency, amount, metadata)
		VALUES ($1, $2, $3, $4)
	`, billID, currency, amount, metadataBytes)
	if err != nil {
		return fmt.Errorf("failed to insert line item: %w", err)
	}

	// 3. Update the bill's total amount in the metadata
	var billMetadata model.BillMetadata
	if len(currentMetadataJSON) > 0 {
		if err := json.Unmarshal(currentMetadataJSON, &billMetadata); err != nil {
			return fmt.Errorf("failed to unmarshal current metadata for update: %w", err)
		}
	}
	if billMetadata.TotalAmounts == nil {
		billMetadata.TotalAmounts = make(map[string]int64)
	}
	billMetadata.TotalAmounts[currency] += amount

	updatedMetadataJSON, err := json.Marshal(billMetadata)
	if err != nil {
		return fmt.Errorf("failed to marshal updated metadata for update: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE bills
		SET metadata = $2
		WHERE bill_id = $1
	`, billID, updatedMetadataJSON)
	if err != nil {
		return fmt.Errorf("failed to update bill metadata: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}
