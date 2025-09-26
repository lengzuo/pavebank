package fee

import (
	"context"
	"testing"

	"encore.app/fee/model"
	temporal "encore.app/fee/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAddLineItem(t *testing.T) {
	mockTemporalClient := &mockTemporalClient{}

	service := &Service{
		client: mockTemporalClient,
	}

	billID := "test-bill-id"
	params := &AddLineItemParams{
		Amount:      100,
		Currency:    "USD",
		Description: "Test item",
	}

	signal := temporal.AddLineItemSignalRequest{
		Amount:   params.Amount,
		Currency: model.USD,
		BillID:   billID,
		Metadata: &model.LineItemMetadata{Description: params.Description},
	}

	// Mock expectations
	mockTemporalClient.On(
		"SignalWorkflow",
		mock.Anything, // context
		temporal.BillCycleWorkflowID(billID),
		"",
		temporal.AddLineItemSignal,
		signal,
	).Return(nil)

	resp, err := service.AddLineItem(context.Background(), billID, params)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, params.Amount, resp.Amount)
	assert.Equal(t, params.Currency, resp.Currency)
	assert.Equal(t, billID, resp.BillID)
	assert.Equal(t, params.Description, resp.Description)

	mockTemporalClient.AssertExpectations(t)
}
