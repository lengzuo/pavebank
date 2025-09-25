package temporal

import (
	"time"

	"encore.app/fee/model"
	"encore.dev/rlog"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	AddLineItemSignal = "add-line-item"
	CloseBillSignal   = "close-bill"
)

var (
	startToCloseTimeout       = 1 * time.Minute
	maxRetryAttempt     int32 = 10
)

type BillLifecycleWorkflowRequest struct {
	BillID           string
	PolicyType       model.PolicyType
	BillingPeriodEnd time.Time
}

type AddLineItemSignalRequest struct {
	LineItemID string // for idempotency
	Amount     int64
	Currency   model.Currency
	BillID     string
	Metadata   *model.LineItemMetadata
}

type ClosedBillRequest struct {
	BillID string
}

func BillLifecycleWorkflow(ctx workflow.Context, req *BillLifecycleWorkflowRequest) (*model.BillSummary, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: startToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        maxRetryAttempt,
			NonRetryableErrorTypes: []string{billNotFound, billClosed},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var activities *Activities

	// 1. Create bill entry in the database first
	if err := workflow.ExecuteActivity(ctx, activities.CreateBill, req.BillID, req.PolicyType).Get(ctx, nil); err != nil {
		rlog.Error("failed to create bill in database, failing workflow", "error", err, "bill_id", req.BillID)
		return nil, err
	}

	// Setup channels for signals and timer
	signalChan := workflow.GetSignalChannel(ctx, AddLineItemSignal)
	closeChan := workflow.GetSignalChannel(ctx, CloseBillSignal)

	// Idempotency tracking for line items
	processedLineItems := make(map[string]bool)

	// Timer for automatic bill closure
	var timerFired bool
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	durationUntilEnd := req.BillingPeriodEnd.Sub(workflow.Now(ctx))
	timerFuture := workflow.NewTimer(timerCtx, durationUntilEnd)

	workflowCompleted := false
	for !workflowCompleted {
		selector := workflow.NewSelector(ctx)

		// Listen for AddLineItem signals
		selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
			var signal AddLineItemSignalRequest
			c.Receive(ctx, &signal)

			// Idempotency Check
			if processedLineItems[signal.LineItemID] {
				workflow.GetLogger(ctx).Info("Duplicate AddLineItem signal received, ignoring.", "LineItemID", signal.LineItemID)
				return // Ignore duplicate signal
			}
			processedLineItems[signal.LineItemID] = true

			workflow.GetLogger(ctx).Info("Received add line item signal", "LineItemID", signal.LineItemID)
			err := workflow.ExecuteActivity(ctx, activities.AddLineItem, signal.BillID, signal.Currency, signal.Amount, signal.Metadata).Get(ctx, nil)
			if err != nil {
				// If adding a line item fails after retries, log it.
				// Depending on business requirements, we might want to fail the workflow or send an alert.
				// For now, we log and continue, as one failed item might not need to stop the whole bill.
				workflow.GetLogger(ctx).Error("Failed to add line item after all retries.", "Error", err, "LineItemID", signal.LineItemID)
			} else {
				workflow.GetLogger(ctx).Debug("Add line item activity completed.", "LineItemID", signal.LineItemID)
			}
		})

		// Listen for an explicit CloseBill signal
		selector.AddReceive(closeChan, func(c workflow.ReceiveChannel, more bool) {
			var signal ClosedBillRequest
			c.Receive(ctx, &signal)
			workflow.GetLogger(ctx).Info("Received explicit CloseBillSignal.")
			workflowCompleted = true
		})

		// Listen for the billing period end timer
		selector.AddFuture(timerFuture, func(f workflow.Future) {
			workflow.GetLogger(ctx).Info("Billing period timer fired.")
			timerFired = true
			workflowCompleted = true
		})

		selector.Select(ctx)
	}

	// If the workflow is completing before the timer fired (e.g. via explicit signal), cancel the timer.
	if !timerFired {
		cancelTimer()
	}

	// --- Close the Bill ---
	// This logic is now outside the loop and runs if the loop was exited by either the timer or an explicit signal.
	workflow.GetLogger(ctx).Info("Proceeding to close bill.", "BillID", req.BillID)
	if err := workflow.ExecuteActivity(ctx, activities.CloseBill, req.BillID).Get(ctx, nil); err != nil {
		// If closing the bill fails, the workflow must fail to prevent incorrect financial state.
		workflow.GetLogger(ctx).Error("Failed to close bill, failing workflow.", "Error", err, "BillID", req.BillID)
		return nil, err
	}
	workflow.GetLogger(ctx).Info("Bill closed successfully.", "BillID", req.BillID)

	// After the bill is closed, get the final summary.
	var billSummary model.BillSummary
	err := workflow.ExecuteActivity(ctx, activities.GetBillSummary, req.BillID).Get(ctx, &billSummary)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to get final bill summary, failing workflow.", "Error", err, "BillID", req.BillID)
		return nil, err
	}

	return &billSummary, nil
}
