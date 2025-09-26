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

func TestGetBills_OPEN(t *testing.T) {
	mockDB := &mocks.DB{}
	mockTemporalClient := &mockTemporalClient{}

	service := &Service{
		db:     mockDB,
		client: mockTemporalClient,
	}

	params := &GetBillsParams{
		Status: "OPEN",
		Limit:  10,
	}

	bills := []*model.BillDetail{
		{
			BillID:     "test-bill-id",
			Status:     "OPEN",
			PolicyType: "USAGE_BASED",
			CreatedAt:  time.Now(),
		},
	}

	// Mock expectations
	mockDB.On("GetBills", mock.Anything, model.BillStatusOpen, params.Limit, mock.Anything).Return(bills, false, nil)
	mockTemporalClient.On(
		"QueryWorkflow",
		mock.Anything,
		"bill-test-bill-id",
		"",
		temporal.QueryBillTotal,
		mock.Anything,
	).Return(&mockValue{val: map[string]int64{"USD": 100}}, nil)

	resp, err := service.GetBills(context.Background(), params)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.HasMore)
	assert.Len(t, resp.Bills, 1)
	assert.Equal(t, "test-bill-id", resp.Bills[0].BillID)
	assert.Len(t, resp.Bills[0].TotalCharges, 1)
	assert.Equal(t, "USD", resp.Bills[0].TotalCharges[0].Currency)
	assert.Equal(t, int64(100), resp.Bills[0].TotalCharges[0].TotalAmount)

	mockDB.AssertExpectations(t)
	mockTemporalClient.AssertExpectations(t)
}

type mockValue struct {
	val interface{}
}

func (v *mockValue) Get(valPtr interface{}) error {
	if val, ok := v.val.(map[string]int64); ok {
		p, ok := valPtr.(*map[string]int64)
		if ok {
			*p = val
		}
	}
	return nil
}

func (v *mockValue) HasValue() bool {
	return v.val != nil
}

func TestGetBills_CLOSED(t *testing.T) {
	mockDB := &mocks.DB{}
	mockTemporalClient := &mockTemporalClient{}

	service := &Service{
		db:     mockDB,
		client: mockTemporalClient,
	}

	params := &GetBillsParams{
		Status: "OPEN",
		Limit:  10,
	}

	bills := []*model.BillDetail{
		{
			BillID:     "test-bill-id",
			Status:     "CLOSED",
			PolicyType: "USAGE_BASED",
			CreatedAt:  time.Now(),
			TotalCharges: []model.TotalSummary{{
				Currency:    "USD",
				TotalAmount: int64(100),
			}},
		},
	}

	// Mock expectations
	mockDB.On("GetBills", mock.Anything, model.BillStatusOpen, params.Limit, mock.Anything).Return(bills, false, nil)

	resp, err := service.GetBills(context.Background(), params)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.False(t, resp.HasMore)
	assert.Len(t, resp.Bills, 1)
	assert.Equal(t, "test-bill-id", resp.Bills[0].BillID)
	assert.Len(t, resp.Bills[0].TotalCharges, 1)
	assert.Equal(t, "USD", resp.Bills[0].TotalCharges[0].Currency)
	assert.Equal(t, int64(100), resp.Bills[0].TotalCharges[0].TotalAmount)

	mockDB.AssertExpectations(t)
	mockTemporalClient.AssertExpectations(t)
}
