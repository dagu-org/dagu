// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/logger"
	"github.com/dagucloud/dagu/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/internal/cmn/stringutil"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/runtime/transform"
	"github.com/spf13/cobra"
)

// Enqueue returns the cobra command for queueing a DAG-run.
func Enqueue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "enqueue [flags] <DAG definition> [-- param1 param2 ...]",
			Short: "Enqueue a DAG-run to the queue.",
			Long: `Enqueue a DAG-run to the queue.

Examples:
	dagu enqueue --run-id=run_id my_dag -- P1=foo P2=bar
	dagu enqueue --name my_custom_name my_dag.yaml -- P1=foo P2=bar
`,
			Args: cobra.MinimumNArgs(1),
		}, enqueueFlags, runEnqueue,
	)
}

var enqueueFlags = []commandLineFlag{paramsFlag, nameFlag, dagRunIDFlag, queueFlag, labelsFlag, tagsFlag, defaultWorkingDirFlag, triggerTypeFlag, scheduleTimeFlag}

func runEnqueue(ctx *Context, args []string) error {
	if ctx.IsRemote() {
		return remoteRunEnqueue(ctx, args)
	}
	runID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get Run ID: %w", err)
	}

	if runID == "" {
		runID, err = genRunID()
		if err != nil {
			return fmt.Errorf("failed to generate Run ID: %w", err)
		}
	} else if err := validateRunID(runID); err != nil {
		return fmt.Errorf("invalid Run ID: %w", err)
	}

	queueOverride, err := ctx.StringParam("queue")
	if err != nil {
		return fmt.Errorf("failed to get queue override: %w", err)
	}

	dag, _, err := loadDAGWithParams(ctx, args, false)
	if err != nil {
		return err
	}

	if queueOverride != "" {
		dag.Queue = queueOverride
	}

	if err := parseAndAppendLabels(ctx, dag); err != nil {
		return err
	}

	triggerType, err := parseTriggerTypeParam(ctx)
	if err != nil {
		return err
	}

	scheduleTime, err := parseScheduleTimeParam(ctx)
	if err != nil {
		return err
	}

	return enqueueDAGRun(ctx, dag, runID, triggerType, scheduleTime)
}

// enqueueDAGRun enqueues a dag-run to the queue.
// The DAG location is cleared to allow concurrent queued runs (location is used
// for unix pipe generation which would prevent parallel execution).
func enqueueDAGRun(ctx *Context, dag *core.DAG, dagRunID string, triggerType core.TriggerType, scheduleTime string) error {
	dag.Location = ""

	if !ctx.Config.Queues.Enabled {
		return fmt.Errorf("queues are disabled in configuration")
	}

	logFile, err := ctx.GenLogFileName(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to generate log file name: %w", err)
	}

	dagRun := exec.NewDAGRunRef(dag.Name, dagRunID)

	if _, err = ctx.DAGRunStore.FindAttempt(ctx, dagRun); err == nil {
		return fmt.Errorf("DAG %q with ID %q already exists", dag.Name, dagRunID)
	}
	artifactDir, err := ctx.GenArtifactDir(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to generate artifact directory: %w", err)
	}

	att, err := ctx.DAGRunStore.CreateAttempt(ctx.Context, dag, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}

	opts := []transform.StatusOption{
		transform.WithLogFilePath(logFile),
		transform.WithArchiveDir(artifactDir),
		transform.WithAttemptID(att.ID()),
		transform.WithPreconditions(dag.Preconditions),
		transform.WithQueuedAt(stringutil.FormatTime(time.Now())),
		transform.WithHierarchyRefs(
			exec.NewDAGRunRef(dag.Name, dagRunID),
			exec.DAGRunRef{},
		),
		transform.WithTriggerType(triggerType),
	}

	if scheduleTime != "" {
		opts = append(opts, transform.WithScheduleTime(scheduleTime))
	}

	dagStatus := transform.NewStatusBuilder(dag).Create(dagRunID, core.Queued, 0, time.Time{}, opts...)

	if err := att.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}

	if err := att.Write(ctx.Context, dagStatus); err != nil {
		_ = att.Close(ctx.Context)
		return fmt.Errorf("failed to save status: %w", err)
	}

	closeErr := att.Close(ctx.Context)
	if closeErr != nil {
		logger.Warn(ctx.Context, "Failed to close queued status before enqueue",
			tag.Error(closeErr))
	}

	if err := ctx.QueueStore.Enqueue(ctx.Context, dag.ProcGroup(), exec.QueuePriorityLow, dagRun); err != nil {
		if closeErr != nil {
			return errors.Join(
				fmt.Errorf("failed to close run: %w", closeErr),
				fmt.Errorf("failed to enqueue dag-run: %w", err),
			)
		}
		return fmt.Errorf("failed to enqueue dag-run: %w", err)
	}

	logger.Info(ctx.Context, "Enqueued dag-run",
		tag.DAG(dag.Name),
		tag.RunID(dagRunID),
		slog.Any("params", dag.Params),
	)

	return nil
}
