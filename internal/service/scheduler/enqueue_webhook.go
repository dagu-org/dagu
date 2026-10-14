// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/dagrun"
	"github.com/dagucloud/dagu/v2/internal/intake"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/persis"
	"github.com/dagucloud/dagu/v2/internal/publicwebhook"
	queuedomain "github.com/dagucloud/dagu/v2/internal/queue"
	"github.com/dagucloud/dagu/v2/internal/spec"
)

// EnqueueWebhookRun enqueues a webhook-triggered run while preserving the same
// runtime-param semantics as direct webhook execution.
//
// The function is idempotent: if a run with the same ID already exists, it
// returns nil. An error wrapping [publicwebhook.ErrInvalidRun] means the DAG
// or the params cannot start a run.
func EnqueueWebhookRun(
	ctx context.Context,
	dagRunRepository *persis.DAGRunRepository,
	queueStore queuedomain.QueueStore,
	baseLogDir string,
	baseArtifactDir string,
	baseConfig string,
	workspaceBaseConfigDir string,
	definitionID string,
	dag *ir.DAG,
	runID string,
	params string,
	profileName string,
	now time.Time,
) error {
	dagRun := ir.NewDAGRunRef(dag.Name, runID)

	if _, err := dagRunRepository.FindAttempt(ctx, dagRun); err == nil {
		logger.Info(ctx, "Webhook run already exists; skipping",
			tag.DAG(dag.Name),
			tag.RunID(runID),
		)
		return nil
	} else if !errors.Is(err, dagrun.ErrDAGRunIDNotFound) {
		return fmt.Errorf("failed to check existing webhook run: %w", err)
	}

	fullDAG, err := rehydrateExecutionDAG(ctx, dag, params, baseConfig, workspaceBaseConfigDir)
	if err != nil {
		return fmt.Errorf("%w: %w", publicwebhook.ErrInvalidRun, err)
	}
	if fullDAG == nil {
		return fmt.Errorf("failed to load full DAG for webhook enqueue: DAG is nil")
	}
	if len(fullDAG.BuildErrors) > 0 {
		return fmt.Errorf("%w: %w", publicwebhook.ErrInvalidRun, errors.Join(fullDAG.BuildErrors...))
	}
	if err := spec.ValidateStartParams(fullDAG.DefaultParams, spec.StartParamInput{RawParams: params}); err != nil {
		return fmt.Errorf("%w: %w", publicwebhook.ErrInvalidRun, err)
	}

	dagCopy := fullDAG.Clone()
	dagCopy.Location = ""

	_, err = intake.EnqueueRun(ctx, intake.QueueRequest{
		DAGRunRepository: dagRunRepository,
		QueueStore:       queueStore,
		DAG:              dagCopy,
		DAGRunID:         runID,
		LogBaseDir:       baseLogDir,
		ArtifactBaseDir:  baseArtifactDir,
		TriggerType:      ir.TriggerTypeWebhook,
		ProfileName:      profileName,
		DefinitionID:     definitionID,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		return fmt.Errorf("failed to enqueue webhook run: %w", err)
	}

	logger.Info(ctx, "Webhook run enqueued",
		tag.DAG(dag.Name),
		tag.RunID(runID),
	)

	return nil
}
