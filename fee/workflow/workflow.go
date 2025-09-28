package temporal

import (
	"time"

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
	AddLineItemSignal           = "add-line-item"
	UpdateLineItemSignal        = "update-line-item"
	CloseBillSignal             = "close-bill"
	ContinueAsNewEventThreshold = 500
)

type BillState struct {
	Totals     map[string]int64
	BillID     string
	EventCount int
}

// BillLifecycleWorkflow
func BillLifecycleWorkflow(ctx workflow.Context, req *BillLifecycleWorkflowRequest) (*BillResponse, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: startToCloseTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        maxRetryAttempt,
			NonRetryableErrorTypes: []string{errNotFound},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)
	var activities *Activities

	// 1. Get the correct billing policy from the factory
	policy, err := NewBillingPolicy(req.PolicyType)
	if err != nil {
		return nil, err // Fails the workflow if policy is unknown
	}

	// 2. Initialize state
	var state BillState
	if req.PreviousState != nil {
		state = *req.PreviousState
	} else {
		// Create bill in DB
		if err := workflow.ExecuteActivity(ctx, activities.CreateBill, req.BillID, req.PolicyType).Get(ctx, nil); err != nil {
			return nil, err
		}
		// Get initial state from the policy
		state = policy.GetInitialState(req)
	}

	// Create a query handler for API to query the current total bills before bills is closed
	err = workflow.SetQueryHandler(ctx, QueryBillTotal, func() (map[string]int64, error) {
		return state.Totals, nil
	})
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to register query bill total handler", "error", err)
		return nil, err
	}

	// Setup channels for signals and timer
	addItemSignalChan := workflow.GetSignalChannel(ctx, AddLineItemSignal)
	updateItemSignalChan := workflow.GetSignalChannel(ctx, UpdateLineItemSignal)
	closeChan := workflow.GetSignalChannel(ctx, CloseBillSignal)

	// Timer for automatic bill closure
	var timerFired bool
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	durationUntilEnd := req.BillingPeriodEnd.Sub(workflow.Now(ctx))
	timerFuture := workflow.NewTimer(timerCtx, durationUntilEnd)

	workflowCompleted := false
	for {
		selector := workflow.NewSelector(ctx)

		// Listen for AddLineItem signals
		selector.AddReceive(addItemSignalChan, func(c workflow.ReceiveChannel, more bool) {
			var signal AddLineItemSignalRequest
			c.Receive(ctx, &signal)
			state.EventCount++

			// DELEGATE to the policy
			if policy.HandleAddLineItem(ctx, activities, signal) {
				state.Totals[string(signal.Currency)] += signal.Amount
			}
		})

		// Listen for UpdateLineItem signals
		selector.AddReceive(updateItemSignalChan, func(c workflow.ReceiveChannel, more bool) {
			var signal UpdateLineItemSignalRequest
			c.Receive(ctx, &signal)
			state.EventCount++

			// DELEGATE to the policy
			if lineItem := policy.HandleUpdateLineItem(ctx, activities, signal); lineItem != nil {
				state.Totals[lineItem.Currency] -= lineItem.Amount
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
			// DELEGATE to the policy
			if policy.OnTimerFired(ctx, &state) {
				timerFired = true
				workflowCompleted = true
			}
		})

		selector.Select(ctx)

		// If the workflow is marked for completion (by signal or timer), break the loop.
		// This must be checked *before* the ContinueAsNew condition.
		if workflowCompleted {
			break
		}

		// Check if the event threshold has been reached to continue as new
		if state.EventCount >= ContinueAsNewEventThreshold {
			workflow.GetLogger(ctx).Info("Event threshold reached, continuing as new.", "EventCount", state.EventCount)
			req.PreviousState = &state
			return nil, workflow.NewContinueAsNewError(ctx, BillLifecycleWorkflow, req)
		}
	}

	// If the workflow is completing before the timer fired (e.g. via manual close from API), cancel the timer.
	if !timerFired {
		cancelTimer()
	}

	// --- Close the Bill ---
	// DELEGATE final policy logic before closing
	if err := policy.OnBillClose(ctx, &state); err != nil {
		return nil, err
	}

	// This logic is now outside the loop and runs if the loop was exited by either the timer or an explicit signal.
	if err := workflow.ExecuteActivity(ctx, activities.CloseBillFromState, state).Get(ctx, nil); err != nil {
		// If closing the bill fails, the workflow must fail to prevent incorrect financial state.
		workflow.GetLogger(ctx).Error("Failed to close bill, failing workflow.", "Error", err, "BillID", req.BillID)
		return nil, err
	}
	workflow.GetLogger(ctx).Info("Bill closed successfully.", "BillID", req.BillID)

	// After the billDetail is closed, get the final billDetail.
	var billDetail BillResponse
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

	var billDetail BillResponse
	err := workflow.ExecuteActivity(ctx, activities.GetBillDetail, req.BillID).Get(ctx, &billDetail)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to get final bill summary, failing workflow.", "Error", err, "BillID", req.BillID)
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.GeneratePDFInvoive, req.BillID).Get(ctx, &billDetail)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to generate pdf invoice", "Error", err, "BillID", req.BillID)
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.CreatePaymentLink, req.BillID).Get(ctx, &billDetail)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to create payment link.", "Error", err, "BillID", req.BillID)
		return err
	}

	err = workflow.ExecuteActivity(ctx, activities.SendBillEmail, req.BillID).Get(ctx, &billDetail)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to send email.", "Error", err, "BillID", req.BillID)
		return err
	}

	workflow.GetLogger(ctx).Info("Bill Post-process child workflow completed.", "BillID", req.BillID)
	rlog.Info("Post process completed.")

	return nil
}
