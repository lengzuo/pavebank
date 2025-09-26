package temporal

import (
	"time"

	"encore.app/fee/model"
	"encore.dev"
	"encore.dev/rlog"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var (
	envName                   = encore.Meta().Environment.Name
	BillCycleTaskQueue        = envName + "bill3-lifecycle"
	ClosedBillTaskQueue       = envName + "closed-bill-lifecycle"
	startToCloseTimeout       = 1 * time.Minute
	QueryBillTotal            = "GET_BILL_TOTAL"
	maxRetryAttempt     int32 = 10
)

const (
	AddLineItemSignal = "add-line-item"
	CloseBillSignal   = "close-bill"
)

type BillState struct {
	Totals map[string]int64
	BillID string
}

// BillLifecycleWorkflow
func BillLifecycleWorkflow(ctx workflow.Context, req *BillLifecycleWorkflowRequest) (*model.BillDetail, error) {
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
		workflow.GetLogger(ctx).Error("Failed to create bill in database, failing workflow", "error", err, "bill_id", req.BillID)
		return nil, err
	}

	// State as the bill allows for the progressive accrual of fees per currency
	state := BillState{
		BillID: req.BillID,
		Totals: make(map[string]int64),
	}

	// Create a query handler for API to query the current total bills before bills is closed
	err := workflow.SetQueryHandler(ctx, QueryBillTotal, func() (map[string]int64, error) {
		return state.Totals, nil
	})
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to register query bill total handler", "error", err)
		return nil, err
	}

	// Setup channels for signals and timer
	signalChan := workflow.GetSignalChannel(ctx, AddLineItemSignal)
	closeChan := workflow.GetSignalChannel(ctx, CloseBillSignal)

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

			// Idempotency to ensure we only insert one entry if the activity error due to worker crash, network issues, or explicit retry policy
			idempotencyKey := UUID()

			err := workflow.ExecuteActivity(ctx, activities.AddLineItem, signal.BillID, signal.Currency, signal.Amount, signal.Metadata, idempotencyKey).Get(ctx, nil)
			if err != nil {
				// If adding a line item fails after retries, log it.
				// Depending on business requirements, we might want to fail the workflow or send an alert.
				// For now, we log and continue, as one failed item might not need to stop the whole bill.
				workflow.GetLogger(ctx).Error("Failed to add line item after all retries.", "Error", err)
			} else {
				workflow.GetLogger(ctx).Debug("Add line item activity completed.", "BillID", signal.BillID)
				state.Totals[string(signal.Currency)] += signal.Amount
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

	// If the workflow is completing before the timer fired (e.g. via manual close from API), cancel the timer.
	if !timerFired {
		cancelTimer()
	}

	// --- Close the Bill ---
	// This logic is now outside the loop and runs if the loop was exited by either the timer or an explicit signal.
	if err := workflow.ExecuteActivity(ctx, activities.CloseBillFromState, state).Get(ctx, nil); err != nil {
		// If closing the bill fails, the workflow must fail to prevent incorrect financial state.
		workflow.GetLogger(ctx).Error("Failed to close bill, failing workflow.", "Error", err, "BillID", req.BillID)
		return nil, err
	}
	workflow.GetLogger(ctx).Info("Bill closed successfully.", "BillID", req.BillID)

	// After the billDetail is closed, get the final billDetail.
	var billDetail model.BillDetail
	err = workflow.ExecuteActivity(ctx, activities.GetBillDetail, req.BillID).Get(ctx, &billDetail)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to get final bill summary, failing workflow.", "Error", err, "BillID", req.BillID)
		return nil, err
	}

	ctx = workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:        BillPostprocessWorkflowID(req.BillID),
		TaskQueue:         ClosedBillTaskQueue,
		ParentClosePolicy: enums.PARENT_CLOSE_POLICY_ABANDON,
	})

	// Only trigger post process if there is a charges required
	if len(billDetail.TotalCharges) > 0 {
		childWorkflow := workflow.ExecuteChildWorkflow(ctx, ClosedBillPostProcessWorkflow, BillClosedPostProcessWorkflowRequest{
			BillID: req.BillID,
		})
		if err := childWorkflow.GetChildWorkflowExecution().Get(ctx, nil); err != nil {
			// This is a serious problem, it means we couldn't even START the child workflow.
			// This is worth logging an error for, but we still shouldn't fail the parent.
			// The bill is closed, which is the main thing.
			workflow.GetLogger(ctx).Error("CRITICAL: Failed to START post-process child workflow. Manual review required.", "Error", err, "BillID", req.BillID)
			// We will NOT return the error, allowing the parent workflow to complete successfully. Trigger alert
		} else {
			workflow.GetLogger(ctx).Info("Successfully started post-process child workflow.", "BillID", req.BillID)
		}
	}

	return &billDetail, nil
}

// ClosedBillPostProcessWorkflow
func ClosedBillPostProcessWorkflow(ctx workflow.Context, req *BillClosedPostProcessWorkflowRequest) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: startToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: maxRetryAttempt,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities

	var billSummary model.BillDetail
	err := workflow.ExecuteActivity(ctx, activities.GetBillDetail, req.BillID).Get(ctx, &billSummary)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to get final bill summary, failing workflow.", "Error", err, "BillID", req.BillID)
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.GeneratePDFInvoive, req.BillID).Get(ctx, &billSummary)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to generate pdf invoice", "Error", err, "BillID", req.BillID)
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.CreatePaymentLink, req.BillID).Get(ctx, &billSummary)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to create payment link.", "Error", err, "BillID", req.BillID)
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.SendBillEmail, req.BillID).Get(ctx, &billSummary)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send email.", "Error", err, "BillID", req.BillID)
		return err
	}

	workflow.GetLogger(ctx).Info("Bill Post-process child workflow completed.", "BillID", req.BillID)
	rlog.Info("Post process completed.")

	return nil
}
