package fee

import (
	"context"
	"testing"
	"time"

	"encore.app/fee/dao/mocks"
	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateBill(t *testing.T) {
	mockTemporalClient := &mockTemporalClient{}
	mockDB := &mocks.DB{}

	service := &Service{
		db:     mockDB,
		client: mockTemporalClient,
	}

	params := &CreateBillParams{
		BillID:           "test-bill-id",
		BillingPeriodEnd: time.Now().Add(5 * time.Minute),
	}

	// Mock expectations
	mockDB.On("IsBillExists", mock.Anything, params.BillID).Return(false, nil).Once()

	mockTemporalClient.On(
		"ExecuteWorkflow",
		mock.Anything, // context
		mock.Anything, // options
		mock.AnythingOfType("func(internal.Context, *temporal.BillLifecycleWorkflowRequest) (*temporal.BillResponse, error)"),
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
	mockDB.AssertExpectations(t)
}
