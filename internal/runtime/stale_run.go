// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

const staleLocalRunError = "process terminated unexpectedly - stale local process detected"

// RepairStaleLocalRun marks an active local run as failed after liveness checks
// have confirmed the local proc file is stale or missing.
func RepairStaleLocalRun(
	ctx context.Context,
	attempt exec.DAGRunAttempt,
	dag *core.DAG,
) (*exec.DAGRunStatus, bool, error) {
	fullStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("read full status: %w", err)
	}
	if !fullStatus.Status.IsActive() {
		return fullStatus, false, nil
	}
	if fullStatus.WorkerID != "" && fullStatus.WorkerID != "local" {
		return fullStatus, false, nil
	}

	repairedStatus := cloneStatusForStaleRunRepair(fullStatus)
	repairedStatus.Status = core.Failed
	repairedStatus.FinishedAt = exec.FormatTime(time.Now())

	if len(repairedStatus.Nodes) == 0 {
		if dag == nil {
			return nil, false, fmt.Errorf("dag is required when rebuilding missing nodes")
		}
		repairedStatus.Nodes = exec.NewNodesFromSteps(dag.Steps)
	}

	for _, node := range repairedStatus.Nodes {
		if node.Status == core.NodeRunning || node.Status == core.NodeNotStarted {
			node.Status = core.NodeFailed
			node.Error = staleLocalRunError
		}
	}

	if err := writeAttemptStatus(ctx, attempt, *repairedStatus); err != nil {
		return nil, false, err
	}

	return repairedStatus, true, nil
}

func cloneStatusForStaleRunRepair(status *exec.DAGRunStatus) *exec.DAGRunStatus {
	if status == nil {
		return nil
	}

	cloned := *status
	if len(status.Nodes) > 0 {
		cloned.Nodes = make([]*exec.Node, 0, len(status.Nodes))
		for _, node := range status.Nodes {
			if node == nil {
				cloned.Nodes = append(cloned.Nodes, nil)
				continue
			}
			nodeCopy := *node
			cloned.Nodes = append(cloned.Nodes, &nodeCopy)
		}
	}

	return &cloned
}

func writeAttemptStatus(ctx context.Context, attempt exec.DAGRunAttempt, status exec.DAGRunStatus) error {
	if err := attempt.Open(ctx); err != nil {
		return fmt.Errorf("open attempt: %w", err)
	}
	defer func() {
		if err := attempt.Close(ctx); err != nil {
			logger.Error(ctx, "Failed to close attempt", tag.Error(err))
		}
	}()

	if err := attempt.Write(ctx, status); err != nil {
		return fmt.Errorf("write status: %w", err)
	}

	return nil
}
