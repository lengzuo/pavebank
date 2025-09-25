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

type BillLifecycleWorkflowRequest struct {
	BillID     string
	PolicyType model.PolicyType
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

func Greeting(ctx workflow.Context, name string) (string, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Second * 5,
	}
	rlog.Debug("ssssss workflow", "id", name)
	ctx = workflow.WithActivityOptions(ctx, options)

	var result string
	err := workflow.ExecuteActivity(ctx, ComposeGreeting, name).Get(ctx, &result)

	return result, err
}

func BillLifecycleWorkflow(ctx workflow.Context, req *BillLifecycleWorkflowRequest) (*model.BillSummary, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        10,
			NonRetryableErrorTypes: []string{billNotFound, billClosed},
		},
	}
	rlog.Debug("BillLifecycleWorkflow workflow", "id", req)
	ctx = workflow.WithActivityOptions(ctx, ao)
	var activities *Activities

	signalChan := workflow.GetSignalChannel(ctx, AddLineItemSignal)
	closeChan := workflow.GetSignalChannel(ctx, CloseBillSignal)

	workflowCompleted := false

	for !workflowCompleted {
		selector := workflow.NewSelector(ctx)

		switch req.PolicyType {
		case model.PolicyTypeUsageBased:
			selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
				var signal AddLineItemSignalRequest
				c.Receive(ctx, &signal)
				rlog.Info("getting add line item signal")
				workflow.ExecuteActivity(ctx, activities.AddLineItem, signal.BillID, signal.Currency, signal.Amount, signal.Metadata).Get(ctx, nil)
				rlog.Debug("add line item completed ", signal.Amount)
			})
		case model.PolicyTypeHourly:
			// Automaitcally collect bill every hours from a separate servcies.
			// Mock the amount and currency for this assignment
			selector.AddFuture(workflow.NewTimer(ctx, time.Hour), func(f workflow.Future) {
				workflow.ExecuteActivity(ctx, activities.AddLineItem, req.BillID, "USD", 100, nil).Get(ctx, nil)
				rlog.Debug("hourly job add line item completed")
			})
		}

		selector.AddReceive(closeChan, func(c workflow.ReceiveChannel, more bool) {
			var signal ClosedBillRequest
			c.Receive(ctx, &signal)
			workflow.ExecuteActivity(ctx, activities.CloseBill, signal.BillID).Get(ctx, nil)
			workflow.GetLogger(ctx).Info("Bill closed, workflow completing.")
			workflowCompleted = true
		})

		selector.Select(ctx)
	}

	// After the loop, update the bill status to be returned
	// This activity call is crucial to get the final state of the bill for the return value
	var billSummary model.BillSummary
	err := workflow.ExecuteActivity(ctx, activities.GetBillSummary, req.BillID).Get(ctx, &billSummary)
	if err != nil {
		return nil, err
	}

	return &billSummary, nil
}
