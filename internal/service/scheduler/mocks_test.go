package scheduler_test

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/service/scheduler"
)

var _ scheduler.EntryReader = (*mockJobManager)(nil)

type mockJobManager struct {
	LoadedDAGs []*core.DAG
}

func newMockJobManager() *mockJobManager {
	return &mockJobManager{}
}

func (er *mockJobManager) Init(_ context.Context) error {
	return nil
}

func (er *mockJobManager) Start(_ context.Context) {
}

func (er *mockJobManager) Stop() {
}

func (er *mockJobManager) DAGs() []*core.DAG {
	return er.LoadedDAGs
}
