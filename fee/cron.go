package fee

import (
	"context"
	"time"

	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"encore.dev/cron"
	"encore.dev/rlog"
	"go.temporal.io/sdk/client"
)

// This cron job runs on the first day of every month to start new billing cycles.
var _ = cron.NewJob("start-monthly-billing", cron.JobConfig{
	Title:    "Start Monthly Billing",
	Schedule: "0 0 1 * *",
	Endpoint: StartMonthlyBilling,
})

// StartMonthlyBilling is the cron job handler for starting new billing cycles.
//
//encore:api private
func StartMonthlyBilling(ctx context.Context) error {
	s, err := initService()
	if err != nil {
		return err
	}

	// In a real-world application, you would fetch the list of active customers from your database.
	// For this example, we'll use a placeholder function.
	customerIDs, err := getAllCustomerIDs(ctx)
	if err != nil {
		rlog.Error("failed to get customer IDs", "error", err)
		return err
	}

	now := time.Now()
	// Start of the next month
	startOfNextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
	// End of the current month is one second before the start of the next month
	billingPeriodEnd := startOfNextMonth.Add(-1 * time.Second)

	for _, customerID := range customerIDs {
		workflowID := temporal.BillCycleWorkflowID(customerID)

		req := &temporal.BillLifecycleWorkflowRequest{
			BillID:           customerID,
			PolicyType:       model.PolicyTypeMonthly,
			BillingPeriodEnd: billingPeriodEnd,
		}

		// Start the workflow.
		_, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: temporal.BillCycleTaskQueue,
		}, temporal.BillLifecycleWorkflow, req)
		if err != nil {
			rlog.Error("failed to start bill lifecycle workflow", "error", err, "customer_id", customerID)
			// Continue to the next customer even if one fails.
			continue
		}
	}

	return nil
}

// // This cron job runs on the first day of every month to close the previous month's bills.
// var _ = cron.NewJob("close-monthly-billing", cron.JobConfig{
// 	Title:    "Close Monthly Billing",
// 	Schedule: "0 0 1 * *",
// 	Endpoint: CloseMonthlyBilling,
// })

// // CloseMonthlyBilling is the cron job handler for closing the previous month's bills.
// //
// //encore:api private
// func CloseMonthlyBilling(ctx context.Context) error {
// 	s, err := initService()
// 	if err != nil {
// 		return err
// 	}
// 	// Get all open bills from the previous month.
// 	// In a real-world application, you would have a more robust way to determine the previous month.
// 	billIDs, _, err := dao.GetBillIDs(ctx, model.BillStatusOpen, model.PolicyTypeMonthly, 1000, time.Now())
// 	if err != nil {
// 		rlog.Error("failed to get open bills", "error", err)
// 		return err
// 	}

// 	for _, billID := range billIDs {
// 		workflowID := fmt.Sprintf("bill-%s", billID)
// 		signal := temporal.ClosedBillRequest{
// 			BillID: billID,
// 		}

// 		// Signal the workflow to close the bill.
// 		err := s.client.SignalWorkflow(ctx, workflowID, "", temporal.CloseBillSignal, signal)
// 		if err != nil {
// 			rlog.Error("failed to signal workflow to close bill", "error", err, "bill_id", billID)
// 			// Continue to the next bill even if one fails.
// 			continue
// 		}
// 	}

// 	return nil
// }

// getAllCustomerIDs is a placeholder function. In a real-world application, this would
// query your database to get a list of all active customers.
func getAllCustomerIDs(_ context.Context) ([]string, error) {
	return []string{"customer-1", "customer-2"}, nil
}
