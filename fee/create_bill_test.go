package fee

import (
	"context"
	"testing"
	"time"

	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateBill(t *testing.T) {
	mockTemporalClient := &mockTemporalClient{}

	service := &Service{
		client: mockTemporalClient,
	}

	params := &CreateBillParams{
		BillID:           "test-bill-id",
		BillingPeriodEnd: time.Now().Add(5 * time.Minute),
	}

	// Mock expectations
	mockTemporalClient.On(
		"ExecuteWorkflow",
		mock.Anything, // context
		mock.Anything, // options
		mock.AnythingOfType("func(internal.Context, *temporal.BillLifecycleWorkflowRequest) (*model.BillDetail, error)"),
		&temporal.BillLifecycleWorkflowRequest{
			BillID:           params.BillID,
			PolicyType:       model.PolicyTypeUsageBased,
			BillingPeriodEnd: params.BillingPeriodEnd,
		},
	).Return(&mockWorkflowRun{}, nil)

	resp, err := service.CreateBill(context.Background(), params)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, params.BillID, resp.BillID)
	assert.Equal(t, string(model.BillStatusOpen), resp.Status)

	mockTemporalClient.AssertExpectations(t)
}
