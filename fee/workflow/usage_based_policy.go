package temporal

import (
	"encore.app/fee/model"
	"go.temporal.io/sdk/workflow"
)

// UsageBasedPolicy implements the logic for a standard usage-based bill.
type UsageBasedPolicy struct{}

func (p *UsageBasedPolicy) GetInitialState(req *BillLifecycleWorkflowRequest) BillState {
	return BillState{
		BillID:     req.BillID,
		Totals:     make(map[string]int64),
		EventCount: 0,
	}
}

func (p *UsageBasedPolicy) HandleAddLineItem(ctx workflow.Context, activities *Activities, signal AddLineItemSignalRequest) bool {
	err := workflow.ExecuteActivity(ctx, activities.AddLineItem, signal.BillID, signal.Currency, signal.Amount, signal.Metadata, signal.LineItemID).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to add line item after all retries.", "Error", err)
		return false // Do not update totals if the activity failed
	}
	workflow.GetLogger(ctx).Debug("Add line item activity completed.", "BillID", signal.BillID)
	return true // Signal to the workflow to update the totals
}

func (p *UsageBasedPolicy) HandleUpdateLineItem(ctx workflow.Context, activities *Activities, signal UpdateLineItemSignalRequest) *model.LineItem {
	var lineItem *model.LineItem
	err := workflow.ExecuteActivity(ctx, activities.UpdateLineItem, signal.BillID, signal.LineItemID, signal.Status).Get(ctx, &lineItem)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to update line item.", "Error", err, "BillID", signal.BillID, "LineItemID", signal.LineItemID)
		return nil
	}
	return lineItem
}

func (p *UsageBasedPolicy) OnBillClose(ctx workflow.Context, state *BillState) error {
	// For usage-based, no special logic is needed before closing.
	// The state is already up-to-date from the signals.
	workflow.GetLogger(ctx).Info("Executing final checks for subscription policy before closing.  For example logic calculate overage")
	return nil
}

func (p *UsageBasedPolicy) OnTimerFired(ctx workflow.Context, state *BillState) bool {
	// For a usage-based bill, the timer firing always means we should close the bill.
	return true
}
