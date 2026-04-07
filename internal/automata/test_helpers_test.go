// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/persis/filedag"
	"github.com/dagucloud/dagu/internal/persis/filedagrun"
	"github.com/dagucloud/dagu/internal/persis/filememory"
	"github.com/dagucloud/dagu/internal/persis/filesession"
	"github.com/dagucloud/dagu/internal/service/eventstore"
	"github.com/stretchr/testify/require"
)

type testAutomataEventStore struct {
	events []*eventstore.Event
}

func (s *testAutomataEventStore) Emit(_ context.Context, event *eventstore.Event) error {
	if event == nil {
		return nil
	}
	s.events = append(s.events, event)
	return nil
}

func (*testAutomataEventStore) Query(context.Context, eventstore.QueryFilter) (*eventstore.QueryResult, error) {
	return &eventstore.QueryResult{}, nil
}

func newTestService(t *testing.T) (*Service, time.Time) {
	t.Helper()

	root := t.TempDir()
	dagsDir := filepath.Join(root, "dags")
	dataDir := filepath.Join(root, "data")
	runsDir := filepath.Join(root, "runs")

	require.NoError(t, os.MkdirAll(dagsDir, 0o750))
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.MkdirAll(runsDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "build-app.yaml"),
		[]byte(testDAGYAML("build-app")),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "run-tests.yaml"),
		[]byte(testDAGYAML("run-tests")),
		0o600,
	))

	cfg := &config.Config{
		Core: config.Core{
			Location: time.UTC,
		},
		Paths: config.PathsConfig{
			DAGsDir:    dagsDir,
			DataDir:    dataDir,
			DAGRunsDir: runsDir,
		},
	}
	memoryStore, err := filememory.New(dagsDir)
	require.NoError(t, err)
	fixedTime := time.Date(2026, time.March, 26, 10, 0, 0, 0, time.UTC)
	svc := New(
		cfg,
		filedag.New(dagsDir, filedag.WithSkipExamples(true)),
		filedagrun.New(runsDir),
		WithClock(func() time.Time { return fixedTime }),
		WithMemoryStore(memoryStore),
	)
	return svc, fixedTime
}

func newTestServiceWithSessionStore(t *testing.T) (*Service, time.Time) {
	t.Helper()

	root := t.TempDir()
	dagsDir := filepath.Join(root, "dags")
	dataDir := filepath.Join(root, "data")
	runsDir := filepath.Join(root, "runs")
	sessionDir := filepath.Join(root, "sessions")

	require.NoError(t, os.MkdirAll(dagsDir, 0o750))
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.MkdirAll(runsDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "build-app.yaml"),
		[]byte(testDAGYAML("build-app")),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "run-tests.yaml"),
		[]byte(testDAGYAML("run-tests")),
		0o600,
	))

	sessionStore, err := filesession.New(sessionDir)
	require.NoError(t, err)
	memoryStore, err := filememory.New(dagsDir)
	require.NoError(t, err)

	cfg := &config.Config{
		Core: config.Core{
			Location: time.UTC,
		},
		Paths: config.PathsConfig{
			DAGsDir:    dagsDir,
			DataDir:    dataDir,
			DAGRunsDir: runsDir,
		},
	}
	fixedTime := time.Date(2026, time.March, 26, 10, 0, 0, 0, time.UTC)
	svc := New(
		cfg,
		filedag.New(dagsDir, filedag.WithSkipExamples(true)),
		filedagrun.New(runsDir),
		WithClock(func() time.Time { return fixedTime }),
		WithSessionStore(sessionStore),
		WithMemoryStore(memoryStore),
	)
	return svc, fixedTime
}

func newTestServiceWithEventStore(t *testing.T) (*Service, time.Time, *testAutomataEventStore) {
	t.Helper()

	root := t.TempDir()
	dagsDir := filepath.Join(root, "dags")
	dataDir := filepath.Join(root, "data")
	runsDir := filepath.Join(root, "runs")

	require.NoError(t, os.MkdirAll(dagsDir, 0o750))
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.MkdirAll(runsDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "build-app.yaml"),
		[]byte(testDAGYAML("build-app")),
		0o600,
	))

	cfg := &config.Config{
		Core: config.Core{
			Location: time.UTC,
		},
		Paths: config.PathsConfig{
			DAGsDir:    dagsDir,
			DataDir:    dataDir,
			DAGRunsDir: runsDir,
		},
	}
	memoryStore, err := filememory.New(dagsDir)
	require.NoError(t, err)
	fixedTime := time.Date(2026, time.March, 26, 10, 0, 0, 0, time.UTC)
	store := &testAutomataEventStore{}
	svc := New(
		cfg,
		filedag.New(dagsDir, filedag.WithSkipExamples(true)),
		filedagrun.New(runsDir),
		WithClock(func() time.Time { return fixedTime }),
		WithMemoryStore(memoryStore),
		WithEventService(eventstore.New(store)),
		WithEventSource(eventstore.Source{Service: eventstore.SourceServiceScheduler, Instance: "test-scheduler"}),
	)
	return svc, fixedTime, store
}

func createTask(t *testing.T, svc *Service, ctx context.Context, name, description, requestedBy string) *Task {
	t.Helper()
	task, err := svc.CreateTask(ctx, name, CreateTaskRequest{
		Description: description,
		RequestedBy: requestedBy,
	})
	require.NoError(t, err)
	return task
}

func automataSpec(allowedDAG string) string {
	return `description: Software development automata
goal: Complete the assigned software work
allowed_dags:
  names:
    - ` + allowedDAG + `
`
}

func automataSpecWithModel(allowedDAG, model string) string {
	return `description: Software development automata
goal: Complete the assigned software work
allowed_dags:
  names:
    - ` + allowedDAG + `
agent:
  model: ` + model + `
`
}

func automataSpecMultiDAGs() string {
	return `description: Software development automata
goal: Complete the assigned software work
allowed_dags:
  names:
    - build-app
    - run-tests
`
}

func automataSpecWithSchedule(allowedDAG, schedule string) string {
	return `description: Software development automata
goal: Complete the assigned software work
schedule: "` + schedule + `"
allowed_dags:
  names:
    - ` + allowedDAG + `
`
}

func serviceAutomataSpec(allowedDAG string) string {
	return `kind: service
description: Software development automata
goal: Complete the assigned software work
standing_instruction: Handle inbound work continuously.
allowed_dags:
  names:
    - ` + allowedDAG + `
`
}

func serviceAutomataSpecWithSchedule(allowedDAG, schedule string) string {
	return `kind: service
description: Software development automata
goal: Complete the assigned software work
standing_instruction: Handle inbound work continuously.
schedule: "` + schedule + `"
allowed_dags:
  names:
    - ` + allowedDAG + `
`
}

func allowedDAGNames(items []AllowedDAGInfo) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func testDAGYAML(name string) string {
	return `name: ` + name + `
description: Example DAG
tags:
  - dev
steps:
  - name: echo
    command: echo hello
`
}

func init() {
	core.RegisterExecutorCapabilities("command", core.ExecutorCapabilities{
		Command:          true,
		MultipleCommands: true,
		Script:           true,
		Shell:            true,
	})
}
