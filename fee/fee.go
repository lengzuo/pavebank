package fee

import (
	"context"
	"fmt"

	temporal "encore.app/fee/workflow"
	"encore.dev"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

var (
	envName            = encore.Meta().Environment.Name
	billCycleTaskQueue = envName + "bill3-lifecycle"
)

//encore:service
type Service struct {
	client client.Client
	worker worker.Worker
}

func initService() (*Service, error) {
	// For local development, we can use the default in-memory client.
	// For production, we would configure this with the actual Temporal cluster address.
	tc, err := client.NewLazyClient(client.Options{
		HostPort:  client.DefaultHostPort,
		Namespace: client.DefaultNamespace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create temporal client: %w", err)
	}

	// Initialize and start the worker using the created client
	w := worker.New(tc, billCycleTaskQueue, worker.Options{})

	w.RegisterWorkflow(temporal.BillLifecycleWorkflow)
	w.RegisterActivity(&temporal.Activities{})

	err = w.Start()
	if err != nil {
		tc.Close()
		return nil, fmt.Errorf("failed to start temporal worker: %v", err)
	}
	return &Service{client: tc, worker: w}, nil
}

func (s *Service) Shutdown(force context.Context) {
	s.client.Close()
	s.worker.Stop() // Stop the worker gracefully
}
