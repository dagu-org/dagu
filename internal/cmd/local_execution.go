// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
)

type localExecutionPreparation struct {
	Attempt exec.DAGRunAttempt
	Proc    exec.ProcHandle
}

type localExecutionErrorRecorder func(error)

func prepareLocalExecution(
	ctx *Context,
	dag *core.DAG,
	dagRunID string,
	root exec.DAGRunRef,
	parent exec.DAGRunRef,
	triggerType core.TriggerType,
	scheduleTime string,
	buildAttempt func(context.Context) (exec.DAGRunAttempt, error),
) (*localExecutionPreparation, error) {
	if dag == nil {
		return nil, fmt.Errorf("dag is required")
	}
	if dagRunID == "" {
		return nil, fmt.Errorf("dag-run ID is required")
	}
	if buildAttempt == nil {
		return nil, fmt.Errorf("attempt builder is required")
	}
	if root.Zero() {
		root = exec.NewDAGRunRef(dag.Name, dagRunID)
	}

	if err := ctx.ProcStore.Lock(ctx, dag.ProcGroup()); err != nil {
		return nil, fmt.Errorf("failed to lock process group: %w", err)
	}
	defer ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	attempt, err := buildAttempt(ctx.Context)
	if err != nil {
		return nil, err
	}
	if attempt == nil {
		return nil, fmt.Errorf("attempt builder returned nil attempt")
	}
	attempt.SetDAG(dag)

	proc, err := ctx.ProcStore.Acquire(ctx, dag.ProcGroup(), exec.ProcMeta{
		StartedAt:    time.Now().Unix(),
		Name:         dag.Name,
		DAGRunID:     dagRunID,
		AttemptID:    attempt.ID(),
		RootName:     root.Name,
		RootDAGRunID: root.ID,
	})
	if err != nil {
		_ = recordPreparedAttemptFailure(ctx, attempt, dag, dagRunID, root, parent, triggerType, scheduleTime, err)
		return nil, fmt.Errorf("failed to acquire process handle: %w", errProcAcquisitionFailed)
	}

	return &localExecutionPreparation{
		Attempt: attempt,
		Proc:    proc,
	}, nil
}

func recordPreparedAttemptFailure(
	ctx *Context,
	attempt exec.DAGRunAttempt,
	dag *core.DAG,
	dagRunID string,
	root exec.DAGRunRef,
	parent exec.DAGRunRef,
	triggerType core.TriggerType,
	scheduleTime string,
	runErr error,
) error {
	if attempt == nil {
		return fmt.Errorf("attempt is required")
	}
	if dag == nil {
		return fmt.Errorf("dag is required")
	}
	if dagRunID == "" {
		return fmt.Errorf("dag-run ID is required")
	}
	if root.Zero() {
		root = exec.NewDAGRunRef(dag.Name, dagRunID)
	}

	logPath, _ := ctx.GenLogFileName(dag, dagRunID)
	opts := []transform.StatusOption{
		transform.WithAttemptID(attempt.ID()),
		transform.WithHierarchyRefs(root, parent),
		transform.WithLogFilePath(logPath),
		transform.WithFinishedAt(time.Now()),
		transform.WithError(runErr.Error()),
		transform.WithWorkerID("local"),
		transform.WithTriggerType(triggerType),
	}
	if scheduleTime != "" {
		opts = append(opts, transform.WithScheduleTime(scheduleTime))
	}
	status := transform.NewStatusBuilder(dag).Create(dagRunID, core.Failed, 0, time.Now(), opts...)

	if err := attempt.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open attempt for failure recording: %w", err)
	}
	defer func() {
		_ = attempt.Close(ctx.Context)
	}()

	if err := attempt.Write(ctx.Context, status); err != nil {
		return fmt.Errorf("failed to write failed status: %w", err)
	}
	return nil
}

func withPreparedLocalExecution(
	ctx *Context,
	dag *core.DAG,
	dagRunID string,
	root exec.DAGRunRef,
	parent exec.DAGRunRef,
	triggerType core.TriggerType,
	scheduleTime string,
	buildAttempt func(context.Context) (exec.DAGRunAttempt, error),
	recordPrepareError localExecutionErrorRecorder,
	run func(exec.DAGRunAttempt) error,
) error {
	prepared, err := prepareLocalExecution(
		ctx,
		dag,
		dagRunID,
		root,
		parent,
		triggerType,
		scheduleTime,
		buildAttempt,
	)
	if err != nil {
		logger.Debug(ctx, "Failed to prepare local execution", tag.Error(err))
		if recordPrepareError != nil && !errors.Is(err, errProcAcquisitionFailed) {
			recordPrepareError(err)
		}
		return err
	}

	prevProc := ctx.Proc
	ctx.Proc = prepared.Proc
	defer func() {
		ctx.Proc = prevProc
		_ = prepared.Proc.Stop(ctx)
	}()

	return run(prepared.Attempt)
}
