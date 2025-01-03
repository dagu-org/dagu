package scheduler

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/robfig/cron/v3"
)

var _ jobCreator = (*mockJobFactory)(nil)

type mockJobFactory struct{}

func (f *mockJobFactory) CreateJob(dag *digraph.DAG, _ time.Time, _ cron.Schedule) job {
	return newMockJob(dag)
}

var _ entryReader = (*mockEntryReader)(nil)

type mockEntryReader struct {
	Entries []*entry
}

func (er *mockEntryReader) Read(_ context.Context, _ time.Time) ([]*entry, error) {
	return er.Entries, nil
}

func (er *mockEntryReader) Start(_ context.Context, _ chan any) error {
	return nil
}

var _ job = (*mockJob)(nil)

type mockJob struct {
	DAG          *digraph.DAG
	Name         string
	RunCount     atomic.Int32
	StopCount    atomic.Int32
	RestartCount atomic.Int32
	Panic        error
}

func newMockJob(dag *digraph.DAG) *mockJob {
	return &mockJob{
		DAG:  dag,
		Name: dag.Name,
	}
}

func (j *mockJob) GetDAG(_ context.Context) *digraph.DAG {
	return j.DAG
}

func (j *mockJob) Start(_ context.Context) error {
	j.RunCount.Add(1)
	if j.Panic != nil {
		panic(j.Panic)
	}
	return nil
}

func (j *mockJob) Stop(_ context.Context) error {
	j.StopCount.Add(1)
	return nil
}

func (j *mockJob) Restart(_ context.Context) error {
	j.RestartCount.Add(1)
	return nil
}

func (j *mockJob) String() string {
	return j.Name
}
