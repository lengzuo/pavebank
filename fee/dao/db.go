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
	"github.com/jackc/pgx/v5"
)

var (
	ErrBillNotFound = errors.New("bill not found")
	ErrBillIsClosed = errors.New("bill is closed")
)

var db = sqldb.NewDatabase("fee", sqldb.DatabaseConfig{
	Migrations: "./migrations",
})

type dbStore struct {
	db *sqldb.Database
}

func New() DB {
	return &dbStore{db: db}
}

// CreateBill inserts a new bill into the database.
func (d *dbStore) CreateBill(ctx context.Context, billID, policyType string) error {
	_, err := d.db.Exec(ctx, `
		INSERT INTO bills (bill_id, policy_type, status, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (bill_id) DO NOTHING;
	`, billID, policyType, model.BillStatusOpen)
	if err != nil {
		return err
	}
	return nil
}

// GetBillStatus retrieves the status of a bill.
func (d *dbStore) GetBillStatus(ctx context.Context, billID string) (model.BillStatus, error) {
	var status model.BillStatus
	err := d.db.QueryRow(ctx, "SELECT status FROM bills WHERE bill_id = $1", billID).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("bill not found")
		}
		return "", err
	}
	return status, nil
}

// InsertLineItem inserts a new line item for a bill.
func (d *dbStore) InsertLineItem(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal line item metadata: %w", err)
	}

	_, err = d.db.Exec(ctx, `
		INSERT INTO line_items (bill_id, currency, amount, metadata)
		VALUES ($1, $2, $3, $4)
	`, billID, currency, amount, metadataBytes)
	if err != nil {
		return err
	}
	return nil
}

// UpdateBillTotal updates the total amount for a bill in a specific currency within the metadata JSONB.
func (d *dbStore) UpdateBillTotal(ctx context.Context, billID, currency string, amount int64) error {
	// Retrieve current metadata
	var currentMetadataJSON []byte
	err := d.db.QueryRow(ctx, `SELECT metadata FROM bills WHERE bill_id = $1`, billID).Scan(&currentMetadataJSON)
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
	_, err = d.db.Exec(ctx, `
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
// func CloseBill(ctx context.Context, billID string) error {
// 	_, err := db.Exec(ctx, `
// 		UPDATE bills
// 		SET status = $1, closed_at = now()
// 		WHERE bill_id = $2
// 	`, model.BillStatusClosed, billID)
// 	if err != nil {
// 		return err
// 	}
// 	rlog.Debug("CloseBill success", "billID", billID)
// 	return nil
// }

// CloseBill updates the status and metadata of a bill to closed.
func (d *dbStore) CloseBill(ctx context.Context, billID string, metadata model.BillMetadata) error {
	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(ctx, `
		UPDATE bills
		SET status = $1, metadata = $2, closed_at = now()
		WHERE bill_id = $3
	`, model.BillStatusClosed, metaBytes, billID)
	if err != nil {
		return err
	}
	rlog.Debug("CloseBill success", "billID", billID)
	return nil
}

// GetBill retrieves a bill's main details.
func (d *dbStore) GetBill(ctx context.Context, billID string) (*model.BillDetail, error) {
	var bill model.BillDetail
	var jsonData []byte
	err := d.db.QueryRow(ctx, `
		SELECT bill_id, status, policy_type, created_at, metadata, closed_at
		FROM bills
		WHERE bill_id = $1 
	`, billID).Scan(&bill.BillID, &bill.Status, &bill.PolicyType, &bill.CreatedAt, &jsonData, &bill.ClosedAt)
	if err != nil {
		return nil, err
	}
	if len(jsonData) > 0 {
		totalBillCharges, err := totalAmount(jsonData)
		if err != nil {
			return nil, err
		}
		bill.TotalCharges = totalBillCharges
	}
	lineItems, err := d.GetLineItemsForBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	bill.LineItems = lineItems
	return &bill, nil
}

// GetLineItemsForBill retrieves all line items for a given bill.
func (d *dbStore) GetLineItemsForBill(ctx context.Context, billID string) ([]model.LineItem, error) {
	rows, err := d.db.Query(ctx, `
		SELECT currency, amount, metadata, created_at, status, line_item_id
		FROM line_items
		WHERE bill_id = $1
		ORDER BY created_at DESC
	`, billID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lineItems []model.LineItem
	for rows.Next() {
		var item model.LineItem
		if err := rows.Scan(&item.Currency, &item.Amount, &item.Metadata, &item.CreatedAt, &item.Status, &item.LineItemID); err != nil {
			return nil, err
		}
		lineItems = append(lineItems, item)
	}
	return lineItems, nil
}

func totalAmount(jsonData []byte) (map[string]int64, error) {
	var billMetadata model.BillMetadata
	if err := json.Unmarshal(jsonData, &billMetadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}
	return billMetadata.TotalAmounts, nil
	// var totalCharges []model.TotalSummary
	// for currency, amount := range billMetadata.TotalAmounts {
	// 	totalCharges = append(totalCharges, model.TotalSummary{
	// 		Currency:      currency,
	// 		TotalAmount:   amount,
	// 		DisplayAmount: model.FormatAmount(amount),
	// 	})
	// }
	// return totalCharges, nil
}

// GetBills retrieves a list of bills filtered by status.
func (d *dbStore) GetBills(ctx context.Context, status model.BillStatus, limit int, cursor time.Time) ([]*model.BillDetail, bool, error) {
	query := `
		SELECT bill_id, status, policy_type, created_at, metadata, closed_at
		FROM bills
		WHERE created_at < $1 AND status = $2 
		ORDER BY created_at DESC LIMIT $3
	`
	args := []any{cursor, status, limit + 1}
	rows, err := d.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	var bills []*model.BillDetail
	for rows.Next() {
		var bill model.BillDetail
		var jsonData []byte
		if err := rows.Scan(&bill.BillID, &bill.Status, &bill.PolicyType, &bill.CreatedAt, &jsonData, &bill.ClosedAt); err != nil {
			return nil, false, err
		}
		if len(jsonData) > 0 {
			totalBillCharges, err := totalAmount(jsonData)
			if err != nil {
				return nil, false, err
			}
			bill.TotalCharges = totalBillCharges
		}
		bills = append(bills, &bill)
	}

	hasMore := len(bills) > limit
	if hasMore {
		bills = bills[:limit]
	}

	return bills, hasMore, nil
}

// GetBillIDs retrieves a bill id from status policy type
func (d *dbStore) GetBillIDs(ctx context.Context, status model.BillStatus, policyType model.PolicyType, limit int, cursor time.Time) ([]string, bool, error) {
	query := `
		SELECT bill_id
		FROM bills
		WHERE created_at < $1 AND status = $2 AND policy_type = $3
		ORDER BY created_at DESC LIMIT $4
	`
	args := []interface{}{cursor, status, policyType, limit + 1}
	rows, err := d.db.Query(ctx, query, args...)
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

func (d *dbStore) AddLineItem(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata, lineItemID string) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal line item metadata: %w", err)
	}
	_, err = d.db.Exec(ctx, `
		INSERT INTO line_items (bill_id, currency, amount, metadata, line_item_id, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (line_item_id) DO NOTHING;
	`, billID, currency, amount, metadataBytes, lineItemID)
	if err != nil {
		return fmt.Errorf("failed to insert line item: %w", err)
	}
	return nil
}

func (d *dbStore) UpdateLineItem(ctx context.Context, billID, lineItemID string, status string) (*model.LineItem, error) {
	var lineItem model.LineItem
	err := d.db.QueryRow(ctx, `
		UPDATE line_items li
		SET status = $1,
			updated_at = now()
		WHERE li.line_item_id = $2
		AND li.bill_id = $3
		AND li.status = 'ACTIVE' 
		RETURNING li.currency, li.amount;
	`, status, lineItemID, billID).Scan(&lineItem.Currency, &lineItem.Amount)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to update line item: %w", err)
	}
	return &lineItem, nil
}

// IsBillExists checks if a bill with the given ID exists in the database.
func (d *dbStore) IsBillExists(ctx context.Context, billID string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM bills WHERE bill_id = $1)"
	err := d.db.QueryRow(ctx, query, billID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		rlog.Error("failed to check if bill exists", "error", err, "bill_id", billID)
		return false, err
	}
	return exists, nil
}

// IsLineItemExists checks if a specific line item exists with a given status.
func (d *dbStore) IsLineItemExists(ctx context.Context, billID, lineItemID, status string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM line_items
			WHERE line_item_id = $1
			  AND bill_id = $2
			  AND status = $3
		)`
	err := d.db.QueryRow(ctx, query, lineItemID, billID, status).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check for line item: %w", err)
	}
	return exists, nil
}

// AddLineItemTx adds a line item within a single database transaction to ensure atomicity.
// func AddLineItemTx(ctx context.Context, billID, currency string, amount int64, metadata *model.LineItemMetadata) (err error) {
// 	tx, err := db.Begin(ctx)
// 	if err != nil {
// 		return fmt.Errorf("could not begin transaction: %w", err)
// 	}
// 	defer func() {
// 		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
// 			rlog.Error("failed to rollback")
// 		}
// 	}()

// 	// 1. Get bill status and metadata in a single query, and lock the row for update.
// 	var status model.BillStatus
// 	var currentMetadataJSON []byte
// 	err = db.QueryRow(ctx, "SELECT status, metadata FROM bills WHERE bill_id = $1 FOR UPDATE", billID).Scan(&status, &currentMetadataJSON)
// 	if err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			return ErrBillNotFound
// 		}
// 		return fmt.Errorf("failed to get bill status: %w", err)
// 	}

// 	if status == model.BillStatusClosed {
// 		return ErrBillIsClosed
// 	}

// 	// 2. Insert the line item
// 	metadataBytes, err := json.Marshal(metadata)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal line item metadata: %w", err)
// 	}
// 	_, err = db.Exec(ctx, `
// 		INSERT INTO line_items (bill_id, currency, amount, metadata)
// 		VALUES ($1, $2, $3, $4)
// 	`, billID, currency, amount, metadataBytes)
// 	if err != nil {
// 		return fmt.Errorf("failed to insert line item: %w", err)
// 	}

// 	// 3. Update the bill's total amount in the metadata
// 	var billMetadata model.BillMetadata
// 	if len(currentMetadataJSON) > 0 {
// 		if err := json.Unmarshal(currentMetadataJSON, &billMetadata); err != nil {
// 			return fmt.Errorf("failed to unmarshal current metadata for update: %w", err)
// 		}
// 	}
// 	if billMetadata.TotalAmounts == nil {
// 		billMetadata.TotalAmounts = make(map[string]int64)
// 	}
// 	billMetadata.TotalAmounts[currency] += amount

// 	updatedMetadataJSON, err := json.Marshal(billMetadata)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal updated metadata for update: %w", err)
// 	}
// 	_, err = tx.Exec(ctx, `
// 		UPDATE bills
// 		SET metadata = $2
// 		WHERE bill_id = $1
// 	`, billID, updatedMetadataJSON)
// 	if err != nil {
// 		return fmt.Errorf("failed to update bill metadata: %w", err)
// 	}

// 	if err := tx.Commit(); err != nil {
// 		return fmt.Errorf("commit failed: %w", err)
// 	}

// 	return nil
// }
