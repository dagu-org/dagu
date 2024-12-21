// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/require"
)

func TestRetryCommand(t *testing.T) {
	t.Run("RetryDAG", func(t *testing.T) {
		th := testSetup(t)

		dagFile := th.DAGFile("retry.yaml")

		// Run a DAG.
		args := []string{"start", `--params="foo"`, dagFile}
		th.RunCommand(t, startCmd(), cmdTest{args: args})

		// Find the request ID.
		cli := th.Client()
		ctx := context.Background()
		status, err := cli.GetStatus(ctx, dagFile)
		require.NoError(t, err)
		require.Equal(t, status.Status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		requestID := status.Status.RequestID

		// Retry with the request ID.
		args = []string{"retry", fmt.Sprintf("--req=%s", requestID), dagFile}
		th.RunCommand(t, retryCmd(), cmdTest{
			args:        args,
			expectedOut: []string{`params=[foo]`},
		})
	})
}
