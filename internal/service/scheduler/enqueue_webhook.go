// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/logger"
	"github.com/dagucloud/dagu/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/internal/cmn/logpath"
	"github.com/dagucloud/dagu/internal/cmn/stringutil"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/runtime/transform"
)

// EnqueueWebhookRun enqueues a webhook-triggered run while preserving the same
// runtime-param semantics as direct webhook execution.
func EnqueueWebhookRun(
	ctx context.Context,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	baseLogDir string,
	baseArtifactDir string,
	baseConfig string,
	dag *core.DAG,
	runID string,
	params string,
	now time.Time,
) error {
	dagRun := exec.NewDAGRunRef(dag.Name, runID)

	if _, err := dagRunStore.FindAttempt(ctx, dagRun); err == nil {
		logger.Info(ctx, "Webhook run already exists; skipping",
			tag.DAG(dag.Name),
			tag.RunID(runID),
		)
		return nil
	}

	fullDAG, err := rehydrateExecutionDAG(ctx, dag, params, baseConfig)
	if err != nil {
		return fmt.Errorf("failed to load full DAG for webhook enqueue: %w", err)
	}
	if fullDAG == nil {
		return fmt.Errorf("failed to load full DAG for webhook enqueue: DAG is nil")
	}

	dagCopy := fullDAG.Clone()
	dagCopy.Location = ""

	logFile, err := logpath.Generate(ctx, baseLogDir, dagCopy.LogDir, dagCopy.Name, runID)
	if err != nil {
		return fmt.Errorf("failed to generate webhook log file name: %w", err)
	}
	artifactDir := ""
	if dagCopy.ArtifactsEnabled() {
		artifactDir, err = logpath.GenerateDir(ctx, baseArtifactDir, dagCopy.Artifacts.Dir, dagCopy.Name, runID)
		if err != nil {
			return fmt.Errorf("failed to generate webhook artifact directory: %w", err)
		}
	}

	att, err := dagRunStore.CreateAttempt(ctx, dagCopy, now, runID, exec.NewDAGRunAttemptOptions{})
	if err != nil {
		return fmt.Errorf("failed to create webhook attempt: %w", err)
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		if rmErr := dagRunStore.RemoveDAGRun(ctx, dagRun); rmErr != nil {
			logger.Error(ctx, "Failed to rollback webhook attempt",
				tag.DAG(dag.Name),
				tag.RunID(runID),
				tag.Error(rmErr),
			)
		}
	}()

	opts := []transform.StatusOption{
		transform.WithLogFilePath(logFile),
		transform.WithArchiveDir(artifactDir),
		transform.WithAttemptID(att.ID()),
		transform.WithPreconditions(dagCopy.Preconditions),
		transform.WithQueuedAt(stringutil.FormatTime(now)),
		transform.WithHierarchyRefs(dagRun, exec.DAGRunRef{}),
		transform.WithTriggerType(core.TriggerTypeWebhook),
	}
	dagStatus := transform.NewStatusBuilder(dagCopy).Create(runID, core.Queued, 0, time.Time{}, opts...)

	if err := att.Open(ctx); err != nil {
		return fmt.Errorf("failed to open webhook attempt: %w", err)
	}
	if err := att.Write(ctx, dagStatus); err != nil {
		_ = att.Close(ctx)
		return fmt.Errorf("failed to write webhook status: %w", err)
	}
	if err := att.Close(ctx); err != nil {
		return fmt.Errorf("failed to close webhook attempt: %w", err)
	}
	if err := queueStore.Enqueue(ctx, dagCopy.ProcGroup(), exec.QueuePriorityLow, dagRun); err != nil {
		return fmt.Errorf("failed to enqueue webhook run: %w", err)
	}

	committed = true

	logger.Info(ctx, "Webhook run enqueued",
		tag.DAG(dag.Name),
		tag.RunID(runID),
	)

	return nil
}
