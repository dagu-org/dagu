// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"testing"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		Tags:           &[]string{"env=prod"},
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
