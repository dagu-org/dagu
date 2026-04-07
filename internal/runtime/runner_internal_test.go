// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExternalStepRetryEnabled(t *testing.T) {
	t.Run("DisabledByDefault", func(t *testing.T) {
		ctx := exec.NewContext(context.Background(), &core.DAG{Name: "test"}, "run-1", "test.log")
		assert.False(t, externalStepRetryEnabled(ctx))
	})

	t.Run("EnabledByProcessEnv", func(t *testing.T) {
		t.Setenv(exec.EnvKeyExternalStepRetry, "1")
		ctx := exec.NewContext(context.Background(), &core.DAG{Name: "test"}, "run-1", "test.log")
		assert.True(t, externalStepRetryEnabled(ctx))
	})

	t.Run("EnabledByExecutionContextEnv", func(t *testing.T) {
		_ = os.Unsetenv(exec.EnvKeyExternalStepRetry)
		ctx := exec.NewContext(
			context.Background(),
			&core.DAG{Name: "test"},
			"run-1",
			"test.log",
			exec.WithEnvVars(exec.EnvKeyExternalStepRetry+"=1"),
		)
		assert.True(t, externalStepRetryEnabled(ctx))
	})
}

func TestRunNodeExecution_ExternalStepRetrySkipsRepeatBookkeeping(t *testing.T) {
	t.Parallel()

	step := core.Step{
		Name: "retrying-step",
		Commands: []core.CommandEntry{
			{Command: "false", CmdWithArgs: "false"},
		},
		RetryPolicy: core.RetryPolicy{
			Limit:    1,
			Interval: 5 * time.Second,
		},
		RepeatPolicy: core.RepeatPolicy{
			RepeatMode: core.RepeatModeWhile,
			Interval:   time.Millisecond,
		},
	}
	plan, err := NewPlan(step)
	require.NoError(t, err)

	node := plan.GetNodeByName(step.Name)
	require.NotNil(t, node)

	logDir := t.TempDir()
	runner := New(&Config{
		DAGRunID: "run-1",
		LogDir:   logDir,
	})
	ctx := NewContext(
		context.Background(),
		&core.DAG{Name: "retry-dag", WorkingDir: logDir},
		"run-1",
		filepath.Join(logDir, "dag.log"),
		exec.WithEnvVars(exec.EnvKeyExternalStepRetry+"=1"),
	)
	require.NoError(t, node.Prepare(ctx, logDir, "run-1"))

	runner.runNodeExecution(ctx, plan, node, nil)

	assert.Equal(t, core.NodeRetrying, node.State().Status)
	assert.Equal(t, 0, node.State().DoneCount)
	assert.Equal(t, 1, node.State().RetryCount)
}
