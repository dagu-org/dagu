// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package scheduler

import (
	"sync/atomic"
	"time"

	"github.com/daguflow/dagu/internal/dag"
)

var _ jobCreator = (*mockJobFactory)(nil)

type mockJobFactory struct{}

func (f *mockJobFactory) CreateJob(workflow *dag.DAG, _ time.Time) job {
	return newMockJob(workflow)
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

func newMockJob(workflow *dag.DAG) *mockJob {
	return &mockJob{
		DAG:  workflow,
		Name: workflow.Name,
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
