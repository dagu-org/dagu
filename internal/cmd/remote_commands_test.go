// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	api "github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewBufferString(body)),
	}
}

func TestToExecStatus_MapsRemoteFieldsExplicitly(t *testing.T) {
	t.Parallel()

	detail := &api.DAGRunDetails{
		Name:           "example",
		DagRunId:       "run-1",
		RootDAGRunName: "example",
		RootDAGRunId:   "run-1",
		Status:         api.Status(core.Running),
		StartedAt:      "2026-04-02T00:00:00Z",
		FinishedAt:     "",
		Log:            "/tmp/example.log",
		Params:         new("P1=foo"),
		WorkerId:       new("worker-a"),
		Labels:         &[]string{"env=prod"},
		Nodes: []api.Node{
			{
				Step: api.Step{
					Name: "step-1",
					Commands: &[]api.CommandEntry{
						{Command: "echo", Args: &[]string{"hello"}},
					},
				},
				Status:    api.NodeStatus(core.NodeRunning),
				StartedAt: "2026-04-02T00:00:01Z",
				Stdout:    "/tmp/stdout",
				Stderr:    "/tmp/stderr",
			},
		},
	}

	status, err := toExecStatus(detail)
	require.NoError(t, err)
	assert.Equal(t, "example", status.Name)
	assert.Equal(t, "run-1", status.DAGRunID)
	assert.Equal(t, core.Running, status.Status)
	assert.Equal(t, "/tmp/example.log", status.Log)
	require.Len(t, status.Nodes, 1)
	assert.Equal(t, "step-1", status.Nodes[0].Step.Name)
	require.Len(t, status.Nodes[0].Step.Commands, 1)
	assert.Equal(t, "echo", status.Nodes[0].Step.Commands[0].Command)
	assert.Equal(t, []string{"hello"}, status.Nodes[0].Step.Commands[0].Args)
}

func TestRemoteStatusValueRejectsNone(t *testing.T) {
	t.Parallel()

	_, err := remoteStatusValue("none")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestBuildRemoteHistoryQueryRejectsMalformedLimit(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{Use: "history"}
	initFlags(command, historyFlags...)
	require.NoError(t, command.Flags().Set("limit", "10foo"))

	ctx := &Context{Command: command}
	_, _, err := buildRemoteHistoryQuery(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be an integer")

	require.NoError(t, command.Flags().Set("limit", "0"))
	_, _, err = buildRemoteHistoryQuery(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "greater than 0")
}

func TestBuildRemoteHistoryQueryParsesMultipleStatuses(t *testing.T) {
	t.Parallel()

	command := &cobra.Command{Use: "history"}
	initFlags(command, historyFlags...)
	require.NoError(t, command.Flags().Set("status", "running,queued"))

	ctx := &Context{Command: command}
	query, limit, err := buildRemoteHistoryQuery(ctx, nil)
	require.NoError(t, err)

	assert.Equal(t, 100, limit)
	assert.Equal(t, []int{int(core.Running), int(core.Queued)}, query.Statuses)
}

func TestRemoteClientListDAGRunsUsesRepeatedStatusParams(t *testing.T) {
	t.Parallel()

	var statusValues []string

	client := &remoteClient{
		baseURL: "http://dagu.test",
		client: &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				statusValues = append([]string(nil), r.URL.Query()["status"]...)
				return jsonResponse(`{"dagRuns":[]}`), nil
			}),
		},
	}

	_, err := client.listDAGRuns(context.Background(), remoteHistoryQuery{
		Statuses: []int{int(core.Running), int(core.Queued)},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "5"}, statusValues)
}

func TestRemoteRunHistoryEmptyResultsStreams(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		wantStdout string
	}{
		{
			name:       "table writes message only to stderr",
			wantStdout: "",
		},
		{
			name:       "json writes empty array to stdout and message to stderr",
			format:     "json",
			wantStdout: "[]\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := &cobra.Command{Use: "history"}
			initFlags(command, historyFlags...)
			if tt.format != "" {
				require.NoError(t, command.Flags().Set("format", tt.format))
			}
			var stderr bytes.Buffer
			command.SetErr(&stderr)

			ctx := &Context{
				Context: context.Background(),
				Command: command,
				Remote: &remoteClient{
					baseURL: "http://dagu.test",
					client: &http.Client{
						Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
							return jsonResponse(`{"dagRuns":[]}`), nil
						}),
					},
				},
			}

			stdout, err := captureStdout(t, func() error {
				return remoteRunHistory(ctx, nil)
			})

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, stdout)
			assert.Equal(t, expectedNoHistoryMessage, stderr.String())
		})
	}
}

func TestWaitForRemoteStopHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	ctx := &Context{
		Context: cancelled,
		Remote: &remoteClient{
			client: &http.Client{Timeout: time.Minute},
		},
	}

	err := waitForRemoteStop(ctx, "example", "run-1")
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestEnrichRemoteHistoryStatusPopulatesErrorAndMetadata(t *testing.T) {
	t.Parallel()

	status := &exec.DAGRunStatus{Name: "example", DAGRunID: "run-1"}
	detail := &api.DAGRunDetails{
		Name:           "example",
		DagRunId:       "run-1",
		RootDAGRunName: "example",
		RootDAGRunId:   "run-1",
		Status:         api.Status(core.Failed),
		WorkerId:       new("worker-a"),
		Labels:         &[]string{"env=prod"},
		Nodes: []api.Node{
			{
				Step:   api.Step{Name: "step-1"},
				Status: api.NodeStatus(core.NodeFailed),
				Error:  new("boom"),
			},
		},
	}

	require.NoError(t, enrichRemoteHistoryStatus(status, detail))
	assert.Equal(t, []string{"env=prod"}, status.Labels)
	assert.Equal(t, "worker-a", status.WorkerID)
	assert.Contains(t, status.Error, "boom")
}
