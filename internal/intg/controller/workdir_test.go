// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/require"
)

func TestRunAllowedDAGDefaultsLocalWorkingDirToControllerWorkspace(t *testing.T) {
	const (
		controllerName = "software_dev"
		dagName       = "workspace-child"
	)

	f := newTestFixture(t, dagName, workingDirProbeDAG(dagName),
		toolCallResponse("run_allowed_dag", controllerRunDAGArgs(dagName)),
	)
	f.putController(controllerName, dagName)
	f.startController(controllerName)

	ref := f.waitForCurrentRun(controllerName, 10*time.Second)
	workspace := f.controllerWorkspace(controllerName)
	require.DirExists(t, workspace)

	storedDAG := f.storedDAG(ref)
	assertSamePath(t, workspace, storedDAG.WorkingDir)
	queuedStatus := f.waitForStatus(ref, core.Queued, 10*time.Second)
	require.Equal(t, core.TriggerTypeController, queuedStatus.TriggerType)

	f.startScheduler(10 * time.Second)
	status := f.waitForStatus(ref, core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)
	require.Equal(t, core.TriggerTypeController, status.TriggerType)

	actualPath := filepath.Join(workspace, "actual_pwd.txt")
	require.FileExists(t, actualPath)
	assertSamePath(t, workspace, readTrimmedFile(t, actualPath))
}

func TestRunAllowedDAGKeepsExplicitWorkingDir(t *testing.T) {
	const (
		controllerName = "software_dev"
		dagName       = "explicit-workdir-child"
	)

	explicitDir := t.TempDir()
	f := newTestFixture(t, dagName, explicitWorkingDirProbeDAG(dagName, explicitDir),
		toolCallResponse("run_allowed_dag", controllerRunDAGArgs(dagName)),
	)
	f.putController(controllerName, dagName)
	f.startController(controllerName)

	ref := f.waitForCurrentRun(controllerName, 10*time.Second)
	workspace := f.controllerWorkspace(controllerName)
	require.DirExists(t, workspace)

	storedDAG := f.storedDAG(ref)
	assertSamePath(t, explicitDir, storedDAG.WorkingDir)
	queuedStatus := f.waitForStatus(ref, core.Queued, 10*time.Second)
	require.Equal(t, core.TriggerTypeController, queuedStatus.TriggerType)

	f.startScheduler(10 * time.Second)
	status := f.waitForStatus(ref, core.Succeeded, 20*time.Second)
	require.Equal(t, core.Succeeded, status.Status)
	require.Equal(t, core.TriggerTypeController, status.TriggerType)

	actualPath := filepath.Join(explicitDir, "actual_pwd.txt")
	require.FileExists(t, actualPath)
	assertSamePath(t, explicitDir, readTrimmedFile(t, actualPath))
	require.NoFileExists(t, filepath.Join(workspace, "actual_pwd.txt"))
}
