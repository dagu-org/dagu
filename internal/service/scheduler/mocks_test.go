package scheduler_test

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/service/scheduler"
)

var _ scheduler.EntryReader = (*mockJobManager)(nil)

type mockJobManager struct {
	StopRestartEntries []*scheduler.ScheduledJob
	LoadedDAGs         []*core.DAG
	EventCh            chan scheduler.DAGChangeEvent
}

func newMockJobManager() *mockJobManager {
	return &mockJobManager{
		EventCh: make(chan scheduler.DAGChangeEvent, 256),
	}
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

func (er *mockJobManager) Events() <-chan scheduler.DAGChangeEvent {
	return er.EventCh
}

func (er *mockJobManager) StopRestartJobs(_ context.Context, _ time.Time) []*scheduler.ScheduledJob {
	return er.StopRestartEntries
}

var _ scheduler.Job = (*mockJob)(nil)

type mockJob struct {
	DAG          *core.DAG
	Name         string
	StopCount    atomic.Int32
	RestartCount atomic.Int32
	Panic        error
}

func (j *mockJob) GetDAG(_ context.Context) *core.DAG {
	return j.DAG
}

func (j *mockJob) Stop(_ context.Context) error {
	j.StopCount.Add(1)
	return nil
}

func (j *mockJob) Restart(_ context.Context) error {
	j.RestartCount.Add(1)
	if j.Panic != nil {
		panic(j.Panic)
	}
	return nil
}

func (j *mockJob) String() string {
	return j.Name
}
