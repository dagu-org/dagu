// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/robfig/cron/v3"
)

var _ jobCreator = (*mockJobFactory)(nil)

type mockJobFactory struct{}

func (f *mockJobFactory) CreateJob(workflow *digraph.DAG, _ time.Time, _ cron.Schedule) job {
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
	DAG          *digraph.DAG
	Name         string
	RunCount     atomic.Int32
	StopCount    atomic.Int32
	RestartCount atomic.Int32
	Panic        error
}

func newMockJob(workflow *digraph.DAG) *mockJob {
	return &mockJob{
		DAG:  workflow,
		Name: workflow.Name,
	}
}

func (j *mockJob) GetDAG() *digraph.DAG {
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
