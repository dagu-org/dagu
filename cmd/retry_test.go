// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRetryCommand(t *testing.T) {
	t.Run("RetryDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		dagFile := testDAGFile("retry.yaml")

		// Run a DAG.
		testRunCommand(t, startCmd(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})

		// Find the request ID.
		cli := setup.Client()
		status, err := cli.GetStatus(dagFile)
		require.NoError(t, err)
		require.Equal(t, status.Status.Status, scheduler.StatusSuccess)
		require.NotNil(t, status.Status)

		requestID := status.Status.RequestID

		// Retry with the request ID.
		testRunCommand(t, retryCmd(), cmdTest{
			args:        []string{"retry", fmt.Sprintf("--req=%s", requestID), dagFile},
			expectedOut: []string{"param is foo"},
		})
	})
}
