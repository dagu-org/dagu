package scheduler

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
)

var _ JobManager = (*mockJobManager)(nil)

type mockJobManager struct {
	Entries []*ScheduledJob
}

func (er *mockJobManager) Next(_ context.Context, _ time.Time) ([]*ScheduledJob, error) {
	return er.Entries, nil
}

func (er *mockJobManager) Start(_ context.Context, _ chan any) error {
	return nil
}

var _ Job = (*mockJob)(nil)

type mockJob struct {
	DAG          *digraph.DAG
	Name         string
	RunCount     atomic.Int32
	StopCount    atomic.Int32
	RestartCount atomic.Int32
	Panic        error
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
