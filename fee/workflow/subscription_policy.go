package temporal

import (
	"encore.app/fee/model"
	"go.temporal.io/sdk/workflow"
)

// SubscriptionPolicy implements logic for a fixed-fee subscription.
type SubscriptionPolicy struct{}

// GetInitialState for SubscriptionPolicy
func (p *SubscriptionPolicy) GetInitialState(req *BillLifecycleWorkflowRequest) BillState {
	// Subscriptions might start with a pre-defined fee.
	initialTotals := make(map[string]int64)
	if req.SubscriptionFee != nil {
		initialTotals[req.SubscriptionFee.Currency] = req.SubscriptionFee.Amount
	}
	return BillState{
		BillID:     req.BillID,
		Totals:     initialTotals,
		EventCount: 0,
	}
}

// HandleAddLineItem for SubscriptionPolicy
// Subscriptions might not allow adding ad-hoc line items.
func (p *SubscriptionPolicy) HandleAddLineItem(ctx workflow.Context, activities *Activities, signal AddLineItemSignalRequest) bool {
	workflow.GetLogger(ctx).Warn("Attempted to add a line item to a subscription-based bill. This is not allowed.")
	// Optionally, execute an activity to record this invalid attempt.
	return false // Do not update totals.
}

// HandleUpdateLineItem for SubscriptionPolicy
func (p *SubscriptionPolicy) HandleUpdateLineItem(ctx workflow.Context, activities *Activities, signal UpdateLineItemSignalRequest) *model.LineItem {
	workflow.GetLogger(ctx).Warn("Attempted to update a line item to a subscription-based bill. This is not allowed.")
	return nil
}

// OnBillClose for SubscriptionPolicy
// The magic happens here.
func (p *SubscriptionPolicy) OnBillClose(ctx workflow.Context, state *BillState) error {
	// Before closing, maybe we need to run an activity to verify the subscription is still active.
	// This is where you'd put that logic.
	workflow.GetLogger(ctx).Info("Executing final checks for subscription policy before closing.")
	return nil
}

// OnTimerFired for SubscriptionPolicy
func (p *SubscriptionPolicy) OnTimerFired(ctx workflow.Context, state *BillState) bool {
	// For a subscription-based bill, the timer firing always means we should close the bill.
	return true
}
