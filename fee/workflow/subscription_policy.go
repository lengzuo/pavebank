package temporal

import (
	"fmt"
	"time"

	"encore.app/fee/model"
	"encore.app/fee/utils"
	"go.temporal.io/sdk/workflow"
)

// SubscriptionPolicy implements logic for a fixed-fee subscription.
type SubscriptionPolicy struct {
	Currency                string
	Amount                  int64
	Description             string
	BillID                  string
	RecurringInterval       time.Duration
	RecurringFeeTimerFuture workflow.Future
	CancelFutureFn          workflow.CancelFunc
}

func NewSubscriptionPolicy(ctx workflow.Context, billID, currency string, recurring RecurringPolicy) (*SubscriptionPolicy, error) {
	recurringTimerCtx, cancelRecurringTimer := workflow.WithCancel(ctx)
	recurringFeeInterval, err := time.ParseDuration(recurring.Interval.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parsed recurring.interval")
	}
	recurringFeeTimerFuture := workflow.NewTimer(recurringTimerCtx, recurringFeeInterval)
	return &SubscriptionPolicy{
		BillID:                  billID,
		Amount:                  recurring.Amount,
		Description:             recurring.Description,
		Currency:                currency,
		RecurringInterval:       recurring.Interval.Duration,
		RecurringFeeTimerFuture: recurringFeeTimerFuture,
		CancelFutureFn:          cancelRecurringTimer,
	}, nil
}

func (p *SubscriptionPolicy) HandleRecurringItem(ctx workflow.Context, activities *Activities, onSuccess func(newAmount int64)) error {
	lineItemID := utils.UUID()
	metadata := &model.LineItemMetadata{Description: p.Description}
	err := workflow.ExecuteActivity(ctx, activities.AddLineItem, p.BillID, p.Amount, metadata, lineItemID).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to recurring add line item after all retries.", "Error", err)
		return err
	}
	workflow.GetLogger(ctx).Debug("Recurring add line item activity completed.", "BillID", p.BillID)
	onSuccess(p.Amount)
	p.RecurringFeeTimerFuture = workflow.NewTimer(ctx, p.RecurringInterval)
	return nil
}

// HandleAddLineItem for SubscriptionPolicy
// Subscriptions might not allow adding ad-hoc line items.
func (p *SubscriptionPolicy) HandleAddLineItem(ctx workflow.Context, activities *Activities, signal AddLineItemSignalRequest) bool {
	workflow.GetLogger(ctx).Warn("Attempted to add a line item to a subscription-based bill. This is not allowed.")
	return false
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
	p.CancelFutureFn()
	// This is where you'd put that logic.
	workflow.GetLogger(ctx).Info("Executing final checks for subscription policy before closing.")
	return nil
}

// OnTimerFired for SubscriptionPolicy
func (p *SubscriptionPolicy) OnTimerFired(ctx workflow.Context, state *BillState) bool {
	// For a subscription-based bill, the timer firing always means we should close the bill.
	return true
}

func (p *SubscriptionPolicy) RecurringFuture() workflow.Future {
	return p.RecurringFeeTimerFuture
}
