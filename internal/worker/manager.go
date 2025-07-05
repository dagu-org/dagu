package worker

import (
	"context"
	"sync"
)

type WorkerManager struct {
	workers []*Worker
	lock    sync.Mutex
}

func NewWorkerManager() *WorkerManager {
	return &WorkerManager{}
}

// AddWorker adds a new worker to the manager's list of workers.
func (wm *WorkerManager) AddWorker(worker *Worker) {
	wm.workers = append(wm.workers, worker)
}

type managerKey struct{}

// WithWorkerManager adds a WorkerManager to the context.
func WithWorkerManager(ctx context.Context, wm *WorkerManager) context.Context {
	return context.WithValue(ctx, managerKey{}, wm)
}

// WorkerManagerFromContext retrieves the WorkerManager from the context, if it exists.
func WorkerManagerFromContext(ctx context.Context) (*WorkerManager, bool) {
	wm, ok := ctx.Value(managerKey{}).(*WorkerManager)
	return wm, ok
}
