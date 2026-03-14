// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/spf13/cobra"
)

func FinalizeFailure() *cobra.Command {
	cmd := NewCommand(
		&cobra.Command{
			Use:    "finalize-failure [flags] <DAG name or file>",
			Short:  "Execute deferred terminal failure handling for a failed DAG-run",
			Args:   cobra.ExactArgs(1),
			Hidden: true,
		}, finalizeFailureFlags, runFinalizeFailure,
	)
	return cmd
}

var finalizeFailureFlags = []commandLineFlag{dagRunIDFlagRetry, retryWorkerIDFlag}

func runFinalizeFailure(ctx *Context, args []string) error {
	dagRunID, _ := ctx.StringParam("run-id")
	workerID := getWorkerID(ctx)

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	ref := exec.NewDAGRunRef(name, dagRunID)
	attempt, err := ctx.DAGRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRunID, err)
	}

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}
	if status.Status != core.Failed {
		return fmt.Errorf("latest attempt status is %s, expected failed", status.Status)
	}
	if status.FailureFinalizedAt != "" {
		logger.Info(ctx, "Deferred failure handling already finalized",
			tag.DAG(name),
			tag.RunID(dagRunID),
			tag.AttemptID(status.AttemptID),
		)
		return nil
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from record: %w", err)
	}

	dag, err = restoreDAGFromStatus(ctx.Context, dag, status)
	if err != nil {
		return fmt.Errorf("failed to restore DAG from status: %w", err)
	}

	if len(dag.WorkerSelector) > 0 && workerID == "local" {
		return fmt.Errorf("cannot finalize deferred failure for DAG %q with workerSelector via local CLI dispatch", dag.Name)
	}

	logFile, err := fileutil.OpenOrCreateFile(status.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Info(ctx, "Deferred DAG failure finalization initiated", tag.File(logFile.Name()))

	dr, err := ctx.dagStore(dagStoreConfig{
		SearchPaths:           []string{filepath.Dir(dag.Location)},
		SkipDirectoryCreation: workerID != "local",
	})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	as := ctx.agentStores()

	rootRun := status.Root
	if rootRun.Zero() {
		rootRun = exec.NewDAGRunRef(dag.Name, status.DAGRunID)
	}

	agentInstance := agent.New(
		status.DAGRunID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.DAGRunMgr,
		dr,
		agent.Options{
			FailureFinalizationTarget: status,
			ParentDAGRun:              status.Parent,
			ProgressDisplay:           shouldEnableProgress(ctx),
			WorkerID:                  workerID,
			DAGRunStore:               ctx.DAGRunStore,
			ServiceRegistry:           ctx.ServiceRegistry,
			RootDAGRun:                rootRun,
			PeerConfig:                ctx.Config.Core.Peer,
			TriggerType:               status.TriggerType,
			DefaultExecMode:           ctx.Config.DefaultExecMode,
			AgentConfigStore:          as.ConfigStore,
			AgentModelStore:           as.ModelStore,
			AgentMemoryStore:          as.MemoryStore,
			AgentSkillStore:           as.SkillStore,
			AgentSoulStore:            as.SoulStore,
			AgentRemoteNodeResolver:   as.RemoteNodeResolver,
			RetryFailureWindow:        ctx.Config.Scheduler.RetryFailureWindow,
		},
	)

	return ExecuteAgent(ctx, agentInstance, dag, status.DAGRunID, logFile)
}
