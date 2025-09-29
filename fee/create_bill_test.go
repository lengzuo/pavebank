package fee

import (
	"context"
	"errors"
	"testing"
	"time"

	"encore.app/fee/dao/mocks"
	"encore.app/fee/model"
	"encore.app/fee/utils"
	temporal "encore.app/fee/workflow"
	"encore.dev/beta/errs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// setup sets up the service with mock dependencies for testing.
func setup(t *testing.T) (*Service, *mocks.DB, *mockTemporalClient) {
	mockTemporalClient := &mockTemporalClient{}
	mockDB := &mocks.DB{}

	service := &Service{
		db:     mockDB,
		client: mockTemporalClient,
	}

	return service, mockDB, mockTemporalClient
}

func TestCreateBill_Validation(t *testing.T) {
	futureTime := time.Now().Add(5 * time.Minute)

	testCases := []struct {
		name          string
		params        *CreateBillParams
		expectedError string
	}{
		{
			name: "Missing Bill ID",
			params: &CreateBillParams{
				Currency: "USD",
			},
			expectedError: "bill_id is a required field",
		},
		{
			name: "Invalid Currency",
			params: &CreateBillParams{
				BillID: "test-bill",
			},
			expectedError: "invalid Currency: ",
		},
		{
			name: "Missing Billing Period End",
			params: &CreateBillParams{
				BillID:   "test-bill",
				Currency: "USD",
			},
			expectedError: "billing_period_end is a required field",
		},
		{
			name: "Billing Period End Too Short",
			params: &CreateBillParams{
				BillID:           "test-bill",
				Currency:         "USD",
				BillingPeriodEnd: time.Now(),
			},
			expectedError: "billing_period_end is too short, must be at least 1 minutes ahead",
		},
		{
			name: "Invalid Policy Type",
			params: &CreateBillParams{
				BillID:           "test-bill",
				Currency:         "USD",
				BillingPeriodEnd: futureTime,
			},
			expectedError: "invalid policy",
		},
		{
			name: "Subscription Policy Missing Recurring",
			params: &CreateBillParams{
				BillID:           "test-bill",
				Currency:         "USD",
				BillingPeriodEnd: futureTime,
				PolicyType:       string(model.Subscription),
			},
			expectedError: "recurring is mandatory for policy=SUBSCRIPTION",
		},
		{
			name: "Subscription Policy Invalid Amount",
			params: &CreateBillParams{
				BillID:           "test-bill",
				Currency:         "USD",
				BillingPeriodEnd: futureTime,
				PolicyType:       string(model.Subscription),
				Recurring: &Recurring{
					Amount: 0,
				},
			},
			expectedError: "recurring.amount must be more than zero",
		},
		{
			name: "Subscription Policy Missing Interval",
			params: &CreateBillParams{
				BillID:           "test-bill",
				Currency:         "USD",
				BillingPeriodEnd: futureTime,
				PolicyType:       string(model.Subscription),
				Recurring: &Recurring{
					Amount: 1000,
				},
			},
			expectedError: "recurring.interval must be at provided",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			service, _, _ := setup(t)
			_, err := service.CreateBill(context.Background(), tc.params)

			assert.Error(t, err)
			var errsErr *errs.Error
			assert.True(t, errors.As(err, &errsErr))
			assert.Equal(t, errs.InvalidArgument, errsErr.Code)
			assert.Equal(t, tc.expectedError, errsErr.Message)
		})
	}
}

func TestCreateBill_DuplicateBillID(t *testing.T) {
	service, mockDB, _ := setup(t)
	params := &CreateBillParams{
		BillID:           "duplicate-bill",
		Currency:         "USD",
		PolicyType:       string(model.UsageBased),
		BillingPeriodEnd: time.Now().Add(5 * time.Minute),
	}

	mockDB.On("IsBillExists", mock.Anything, params.BillID).Return(true, nil).Once()

	_, err := service.CreateBill(context.Background(), params)

	assert.Error(t, err)
	var errsErr *errs.Error
	assert.True(t, errors.As(err, &errsErr))
	assert.Equal(t, errs.InvalidArgument, errsErr.Code)
	assert.Equal(t, "duplicate bill id", errsErr.Message)
	mockDB.AssertExpectations(t)
}

func TestCreateBill_DBError(t *testing.T) {
	service, mockDB, _ := setup(t)
	params := &CreateBillParams{
		BillID:           "db-error-bill",
		Currency:         "USD",
		PolicyType:       string(model.UsageBased),
		BillingPeriodEnd: time.Now().Add(5 * time.Minute),
	}
	dbErr := errors.New("database connection failed")

	mockDB.On("IsBillExists", mock.Anything, params.BillID).Return(false, dbErr).Once()

	_, err := service.CreateBill(context.Background(), params)

	assert.Error(t, err)
	assert.Equal(t, dbErr, err)
	mockDB.AssertExpectations(t)
}

func TestCreateBill_TemporalError(t *testing.T) {
	service, mockDB, mockTemporalClient := setup(t)
	params := &CreateBillParams{
		BillID:           "temporal-error-bill",
		Currency:         "USD",
		PolicyType:       string(model.UsageBased),
		BillingPeriodEnd: time.Now().Add(5 * time.Minute),
	}
	temporalErr := errors.New("temporal connection failed")

	mockDB.On("IsBillExists", mock.Anything, params.BillID).Return(false, nil).Once()
	mockTemporalClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, temporalErr).Once()

	_, err := service.CreateBill(context.Background(), params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start workflow")
	mockDB.AssertExpectations(t)
	mockTemporalClient.AssertExpectations(t)
}

func TestCreateBill_Success_UsageBased(t *testing.T) {
	service, mockDB, mockTemporalClient := setup(t)
	params := &CreateBillParams{
		BillID:             "test-usage-based-bill",
		PolicyType:         string(model.UsageBased),
		Currency:           "USD",
		BillingPeriodEnd:   time.Now().Add(5 * time.Minute),
		BillingPeriodStart: time.Now(),
	}

	mockDB.On("IsBillExists", mock.Anything, params.BillID).Return(false, nil).Once()

	expectedWorkflowReq := &temporal.BillLifecycleWorkflowRequest{
		BillID:            params.BillID,
		PolicyType:        model.UsageBased,
		BillingPeriodEnd:  params.BillingPeriodEnd,
		BilingPeriodStart: params.BillingPeriodStart,
		Currency:          params.Currency,
	}

	mockTemporalClient.On(
		"ExecuteWorkflow",
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("func(internal.Context, *temporal.BillLifecycleWorkflowRequest) (*temporal.BillResponse, error)"),
		expectedWorkflowReq,
	).Return(&mockWorkflowRun{}, nil).Once()

	resp, err := service.CreateBill(context.Background(), params)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, params.BillID, resp.BillID)
	assert.Equal(t, string(model.BillStatusOpen), resp.Status)

	mockDB.AssertExpectations(t)
	mockTemporalClient.AssertExpectations(t)
}

func TestCreateBill_Success_Subscription(t *testing.T) {
	service, mockDB, mockTemporalClient := setup(t)
	params := &CreateBillParams{
		BillID:             "test-subscription-bill",
		PolicyType:         string(model.Subscription),
		Currency:           "USD",
		BillingPeriodEnd:   time.Now().Add(10 * time.Minute),
		BillingPeriodStart: time.Now(),
		Recurring: &Recurring{
			Amount:      5000,
			Interval:    utils.Duration{Duration: 30 * 24 * time.Hour},
			Description: "Monthly Subscription",
		},
	}

	mockDB.On("IsBillExists", mock.Anything, params.BillID).Return(false, nil).Once()

	expectedWorkflowReq := &temporal.BillLifecycleWorkflowRequest{
		BillID:            params.BillID,
		PolicyType:        model.Subscription,
		BillingPeriodEnd:  params.BillingPeriodEnd,
		BilingPeriodStart: params.BillingPeriodStart,
		Currency:          params.Currency,
		Recurring: temporal.RecurringPolicy{
			Amount:      params.Recurring.Amount,
			Interval:    params.Recurring.Interval,
			Description: params.Recurring.Description,
		},
	}

	mockTemporalClient.On(
		"ExecuteWorkflow",
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("func(internal.Context, *temporal.BillLifecycleWorkflowRequest) (*temporal.BillResponse, error)"),
		expectedWorkflowReq,
	).Return(&mockWorkflowRun{}, nil).Once()

	resp, err := service.CreateBill(context.Background(), params)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, params.BillID, resp.BillID)
	assert.Equal(t, string(model.BillStatusOpen), resp.Status)

	mockDB.AssertExpectations(t)
	mockTemporalClient.AssertExpectations(t)
}
