// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package autopilot_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/require"
)

func TestRunAllowedDAGDefaultsLocalWorkingDirToAutopilotWorkspace(t *testing.T) {
	const (
		autopilotName = "software_dev"
		dagName       = "workspace-child"
	)

	f := newTestFixture(t, dagName, workingDirProbeDAG(dagName),
		toolCallResponse("run_allowed_dag", autopilotRunDAGArgs(dagName)),
	)
	f.putAutopilot(autopilotName, dagName)
	f.startAutopilot(autopilotName)

	ref := f.waitForCurrentRun(autopilotName, 10*time.Second)
	workspace := f.autopilotWorkspace(autopilotName)
	require.DirExists(t, workspace)

	storedDAG := f.storedDAG(ref)
	assertSamePath(t, workspace, storedDAG.WorkingDir)
	queuedStatus := f.waitForStatus(ref, core.Queued, 10*time.Second)
	require.Equal(t, core.TriggerTypeAutopilot, queuedStatus.TriggerType)

	f.startScheduler(10 * time.Second)
	status := f.waitForStatus(ref, core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)
	require.Equal(t, core.TriggerTypeAutopilot, status.TriggerType)

	actualPath := filepath.Join(workspace, "actual_pwd.txt")
	require.FileExists(t, actualPath)
	assertSamePath(t, workspace, readTrimmedFile(t, actualPath))
}

func TestRunAllowedDAGKeepsExplicitWorkingDir(t *testing.T) {
	const (
		autopilotName = "software_dev"
		dagName       = "explicit-workdir-child"
	)

	explicitDir := t.TempDir()
	f := newTestFixture(t, dagName, explicitWorkingDirProbeDAG(dagName, explicitDir),
		toolCallResponse("run_allowed_dag", autopilotRunDAGArgs(dagName)),
	)
	f.putAutopilot(autopilotName, dagName)
	f.startAutopilot(autopilotName)

	ref := f.waitForCurrentRun(autopilotName, 10*time.Second)
	workspace := f.autopilotWorkspace(autopilotName)
	require.DirExists(t, workspace)

	storedDAG := f.storedDAG(ref)
	assertSamePath(t, explicitDir, storedDAG.WorkingDir)
	queuedStatus := f.waitForStatus(ref, core.Queued, 10*time.Second)
	require.Equal(t, core.TriggerTypeAutopilot, queuedStatus.TriggerType)

	f.startScheduler(10 * time.Second)
	status := f.waitForStatus(ref, core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)
	require.Equal(t, core.TriggerTypeAutopilot, status.TriggerType)

	actualPath := filepath.Join(explicitDir, "actual_pwd.txt")
	require.FileExists(t, actualPath)
	assertSamePath(t, explicitDir, readTrimmedFile(t, actualPath))
	require.NoFileExists(t, filepath.Join(workspace, "actual_pwd.txt"))
}
