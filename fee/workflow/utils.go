package temporal

func BillCycleWorkflowID(billID string) string {
	return "bill-" + billID
}

func BillPostprocessWorkflowID(billID string) string {
	return "bill-" + billID + "-postprocess"
}
