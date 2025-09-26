package fee

import (
	"context"
	"fmt"

	"encore.app/fee/dao"
	temporal "encore.app/fee/workflow"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

//encore:service
type Service struct {
	client  client.Client
	workers []worker.Worker
	db      dao.DB
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
	db := dao.New()

	// Initialize and start the worker using the created client
	billCycleWorker := worker.New(tc, temporal.BillCycleTaskQueue, worker.Options{})
	billCycleWorker.RegisterWorkflow(temporal.BillLifecycleWorkflow)
	billCycleWorker.RegisterActivity(&temporal.Activities{DB: db})
	err = billCycleWorker.Start()
	if err != nil {
		tc.Close()
		return nil, fmt.Errorf("failed to start temporal worker: %v", err)
	}

	closedBillWorker := worker.New(tc, temporal.ClosedBillTaskQueue, worker.Options{})
	closedBillWorker.RegisterWorkflow(temporal.ClosedBillPostProcessWorkflow)
	closedBillWorker.RegisterActivity(&temporal.Activities{DB: db})
	err = closedBillWorker.Start()
	if err != nil {
		tc.Close()
		billCycleWorker.Stop()
		return nil, fmt.Errorf("failed to start closed bill worker: %v", err)
	}
	allWorkers := []worker.Worker{billCycleWorker, closedBillWorker}

	return &Service{client: tc, workers: allWorkers, db: db}, nil
}

func (s *Service) Shutdown(force context.Context) {
	s.client.Close()
	for _, w := range s.workers {
		w.Stop()
	}
}
