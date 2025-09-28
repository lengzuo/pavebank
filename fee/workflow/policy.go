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
	// GetInitialState provides the starting state for a new workflow.
	GetInitialState(req *BillLifecycleWorkflowRequest) BillState

	// HandleAddLineItem processes the logic for adding a line item.
	// It returns true if the workflow's total should be updated.
	HandleAddLineItem(ctx workflow.Context, activities *Activities, signal AddLineItemSignalRequest) (updateTotals bool)

	// HandleUpdateLineItem processes the logic for updating a line item (e.g., voiding).
	// It returns the line item that was updated.
	HandleUpdateLineItem(ctx workflow.Context, activities *Activities, signal UpdateLineItemSignalRequest) *model.LineItem

	// OnBillClose is called just before the bill is finalized.
	// It allows the policy to perform any final calculations or state changes.
	OnBillClose(ctx workflow.Context, state *BillState) error

	// OnTimerFired defines the behavior when the automatic billing timer fires.
	// It returns true if the workflow should complete.
	OnTimerFired(ctx workflow.Context, state *BillState) (shouldComplete bool)
}

// NewBillingPolicy is a factory that returns the appropriate policy implementation.
func NewBillingPolicy(policyType model.PolicyType) (BillingPolicy, error) {
	switch policyType {
	case model.UsageBased:
		return &UsageBasedPolicy{}, nil
	case model.Subscription:
		return &SubscriptionPolicy{}, nil // Example of a future policy
	default:
		return nil, fmt.Errorf("unsupported policy type: %s", policyType)
	}
}
