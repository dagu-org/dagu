// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/stretchr/testify/assert"
)

func TestExecutorCapabilities_Get(t *testing.T) {
	registry := &executorCapabilitiesRegistry{
		caps: make(map[string]ExecutorCapabilities),
	}

	// Test case 1: Registered executor
	caps := ExecutorCapabilities{Command: true, MultipleCommands: true}
	registry.Register("test-executor", caps)
	assert.Equal(t, caps, registry.Get("test-executor"))

	// Test case 2: Unregistered executor should return empty capabilities (strict default)
	assert.Equal(t, ExecutorCapabilities{}, registry.Get("unregistered"))
}

func TestSupportsHelpers(t *testing.T) {
	// Register a test executor with specific capabilities
	caps := ExecutorCapabilities{
		Command:        true,
		Script:         false,
		WorkerSelector: true,
	}
	RegisterExecutorCapabilities("helper-test", caps)

	assert.True(t, SupportsCommand("helper-test"))
	assert.False(t, SupportsScript("helper-test"))
	assert.True(t, SupportsWorkerSelector("helper-test"))

	// Unregistered executor should return false for everything
	assert.False(t, SupportsCommand("unknown"))
	assert.False(t, SupportsScript("unknown"))
	assert.False(t, SupportsShell("unknown"))
}

func TestExecutorCapabilities_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	registry := &executorCapabilitiesRegistry{
		caps: make(map[string]ExecutorCapabilities),
	}

	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("executor-%d", i)
			registry.Register(name, ExecutorCapabilities{Command: true})
			assert.True(t, registry.Get(name).Command)
		}(i)
	}

	for i := range 64 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = registry.Get(fmt.Sprintf("executor-%d", i))
			_ = registry.Get("missing")
		}(i)
	}

	wg.Wait()
	assert.True(t, registry.Get("executor-63").Command)
}

func TestStep_EvalOptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("WithGetEvalOptions", func(t *testing.T) {
		// Register executor with GetEvalOptions callback
		RegisterExecutorCapabilities("eval-opts-test", ExecutorCapabilities{
			Command: true,
			GetEvalOptions: func(_ context.Context, _ Step) []eval.Option {
				return []eval.Option{eval.WithoutExpandShell()}
			},
		})

		step := Step{ExecutorConfig: ExecutorConfig{Type: "eval-opts-test"}}
		opts := step.EvalOptions(ctx)
		assert.Len(t, opts, 1)
	})

	t.Run("WithoutGetEvalOptions", func(t *testing.T) {
		// Register executor without GetEvalOptions
		RegisterExecutorCapabilities("no-eval-opts-test", ExecutorCapabilities{
			Command: true,
		})

		step := Step{ExecutorConfig: ExecutorConfig{Type: "no-eval-opts-test"}}
		opts := step.EvalOptions(ctx)
		assert.Nil(t, opts)
	})

	t.Run("UnregisteredExecutor", func(t *testing.T) {
		step := Step{ExecutorConfig: ExecutorConfig{Type: "unregistered-executor"}}
		opts := step.EvalOptions(ctx)
		assert.Nil(t, opts)
	})
}
