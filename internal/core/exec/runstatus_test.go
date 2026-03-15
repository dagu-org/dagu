// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
)

func TestInitialStatusSnapshotsDAGRetryMetadata(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "retry-dag",
		Queue:    "shared-queue",
		Location: "/tmp/retry-dag.yaml",
		RetryPolicy: &core.DAGRetryPolicy{
			Limit:       3,
			Interval:    2 * time.Minute,
			Backoff:     2.0,
			MaxInterval: 10 * time.Minute,
		},
	}

	status := exec.InitialStatus(dag)

	assert.Equal(t, 3, status.AutoRetryLimit)
	assert.Equal(t, 2*time.Minute, status.AutoRetryInterval)
	assert.Equal(t, 2.0, status.AutoRetryBackoff)
	assert.Equal(t, 10*time.Minute, status.AutoRetryMaxInterval)
	assert.Equal(t, "shared-queue", status.ProcGroup)
	assert.Equal(t, "retry-dag", status.SuspendFlagName)
}
