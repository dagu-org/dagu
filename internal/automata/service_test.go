// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/persis/filedag"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/stretchr/testify/require"
)

func init() {
	core.RegisterExecutorCapabilities("command", core.ExecutorCapabilities{
		Command:          true,
		MultipleCommands: true,
		Script:           true,
		Shell:            true,
	})
}

func TestServiceListInitializesStateAndStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	require.Equal(t, "software-dev", item.Name)
	require.Equal(t, StateIdle, item.State)
	require.Equal(t, "research", item.Stage)
	require.Equal(t, fixedTime, item.LastUpdatedAt)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.NotNil(t, detail.State)
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, "research", detail.State.CurrentStage)
	require.Equal(t, "system", detail.State.StageChangedBy)
	require.Len(t, detail.AllowedDAGs, 1)
	require.Equal(t, "build-app", detail.AllowedDAGs[0].Name)
}

func TestServiceOverrideStagePersistsDeclaredStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	err := svc.OverrideStage(ctx, "software-dev", StageOverrideRequest{
		Stage:       "implement",
		RequestedBy: "tester",
		Note:        "planning complete",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.NotNil(t, detail.State)
	require.Equal(t, "implement", detail.State.CurrentStage)
	require.Equal(t, "tester", detail.State.StageChangedBy)
	require.Equal(t, "planning complete", detail.State.StageNote)
	require.Equal(t, fixedTime, detail.State.StageChangedAt)
}

func TestServiceOverrideStageRejectsUnknownStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	err := svc.OverrideStage(ctx, "software-dev", StageOverrideRequest{Stage: "deploy"})
	require.Error(t, err)
	require.ErrorContains(t, err, `unknown stage "deploy"`)
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
	)
	return svc, fixedTime
}

func automataSpec(allowedDAG string) string {
	return `description: Software development automata
purpose: Ship one development task
goal: Complete the assigned software work
stages:
  - research
  - plan
  - implement
allowedDAGs:
  names:
    - ` + allowedDAG + `
`
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
