package temporal

import (
	"fmt"

	"encore.app/fee/model"
	"go.temporal.io/sdk/workflow"
)

// BillingPolicy defines the contract for different billing strategies.
// Each policy encapsulates the specific logic for handling signals and events
// for a particular type of bill (e.g., usage-based, subscription).
type BillingPolicy interface {
	// HandleAddLineItem processes the logic for adding a line item.
	// It returns true if the workflow's total should be updated.
	HandleAddLineItem(ctx workflow.Context, activities *Activities, signal AddLineItemSignalRequest) (updateTotals bool)

	// HandleUpdateLineItem processes the logic for updating a line item (e.g., voiding).
	// It returns the line item that was updated.
	HandleUpdateLineItem(ctx workflow.Context, activities *Activities, signal UpdateLineItemSignalRequest) *model.LineItem

	// TODO: add descirption
	HandleRecurringItem(ctx workflow.Context, activities *Activities, onSuccess func(newAmount int64)) error

	// OnBillClose is called just before the bill is finalized.
	// It allows the policy to perform any final calculations or state changes.
	OnBillClose(ctx workflow.Context, state *BillState) error

	// OnTimerFired defines the behavior when the automatic billing timer fires.
	// It returns true if the workflow should complete.
	OnTimerFired(ctx workflow.Context, state *BillState) (shouldComplete bool)

	// TODO: add descirption
	RecurringFuture() workflow.Future
}

// NewBillingPolicy is a factory that returns the appropriate policy implementation.
func NewBillingPolicy(ctx workflow.Context, req *BillLifecycleWorkflowRequest) (BillingPolicy, error) {
	switch req.PolicyType {
	case model.UsageBased:
		return NewUsagePolicy(), nil
	case model.Subscription:
		if req.Recurring.Interval.Duration <= 0 {
			return nil, fmt.Errorf("recurring is mandatory for subscription policy")
		}
		return NewSubscriptionPolicy(ctx, req.BillID, req.Currency, req.Recurring)
	default:
		return nil, fmt.Errorf("unsupported policy type: %s", req.PolicyType)
	}
}
