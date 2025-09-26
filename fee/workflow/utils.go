package temporal

import (
	"strings"

	"github.com/google/uuid"
)

func UUID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")
}

func BillCycleWorkflowID(billID string) string {
	return "bill-" + billID
}

func BillPostprocessWorkflowID(billID string) string {
	return "bill-" + billID + "-postprocess"
}
