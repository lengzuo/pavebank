package fee

import (
	"context"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
)

// mockTemporalClient is a mock implementation of the Temporal client.
type mockTemporalClient struct {
	client.Client
	mock.Mock
}

func (m *mockTemporalClient) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	a := m.Called(ctx, options, workflow, args[0])
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(client.WorkflowRun), a.Error(1)
}

func (m *mockTemporalClient) QueryWorkflow(ctx context.Context, workflowID string, runID string, queryType string, args ...interface{}) (converter.EncodedValue, error) {
	a := m.Called(ctx, workflowID, runID, queryType, args)
	if a.Get(0) == nil {
		return nil, a.Error(1)
	}
	return a.Get(0).(converter.EncodedValue), a.Error(1)
}

func (m *mockTemporalClient) SignalWorkflow(ctx context.Context, workflowID string, runID string, signalName string, arg interface{}) error {
	a := m.Called(ctx, workflowID, runID, signalName, arg)
	return a.Error(0)
}

type mockWorkflowRun struct {
	client.WorkflowRun
}

func (m *mockWorkflowRun) Get(ctx context.Context, valuePtr interface{}) error {
	return nil
}

func (m *mockWorkflowRun) GetID() string {
	return "mock-workflow-id"
}

func (m *mockWorkflowRun) GetRunID() string {
	return "mock-run-id"
}
