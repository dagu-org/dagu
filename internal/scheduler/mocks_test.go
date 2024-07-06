package scheduler

import (
	"sync/atomic"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
)

var _ jobCreator = (*mockJobFactory)(nil)

type mockJobFactory struct{}

func (f *mockJobFactory) CreateJob(dg *dag.DAG, next time.Time) job {
	return newMockJob(dg)
}

var _ entryReader = (*mockEntryReader)(nil)

type mockEntryReader struct {
	Entries []*entry
}

func (er *mockEntryReader) Read(_ time.Time) ([]*entry, error) {
	return er.Entries, nil
}

func (er *mockEntryReader) Start(chan any) {}

var _ job = (*mockJob)(nil)

type mockJob struct {
	DAG          *dag.DAG
	Name         string
	RunCount     atomic.Int32
	StopCount    atomic.Int32
	RestartCount atomic.Int32
	Panic        error
}

func newMockJob(dg *dag.DAG) *mockJob {
	return &mockJob{
		DAG:  dg,
		Name: dg.Name,
	}
}

func (j *mockJob) GetDAG() *dag.DAG {
	return j.DAG
}

func (j *mockJob) String() string {
	return j.Name
}

func (j *mockJob) Start() error {
	j.RunCount.Add(1)
	if j.Panic != nil {
		panic(j.Panic)
	}
	return nil
}

func (j *mockJob) Stop() error {
	j.StopCount.Add(1)
	return nil
}

func (j *mockJob) Restart() error {
	j.RestartCount.Add(1)
	return nil
}
