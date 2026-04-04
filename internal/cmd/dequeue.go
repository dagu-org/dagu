// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"errors"
	"fmt"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/spf13/cobra"
)

func Dequeue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "dequeue [flags] <queue-name>",
			Short: "Dequeue a DAG-run from the specified queue",
			Long: `Dequeue a DAG-run from the queue.

Example:
	dagu dequeue default --dag-run=dag_name:my_dag_run_id
	dagu dequeue default
`,
			Args: cobra.ExactArgs(1),
		}, dequeueFlags, runDequeue,
	)
}

var dequeueFlags = []commandLineFlag{paramsFlag, dagRunFlagDequeue}

func runDequeue(ctx *Context, args []string) error {
	if ctx.IsRemote() {
		return remoteRunDequeue(ctx, args)
	}
	queueName := args[0]

	// Get dag-run reference from the context
	dagRunRef, _ := ctx.StringParam("dag-run")
	if dagRunRef == "" {
		return dequeueFirst(ctx, queueName)
	}

	dagRun, err := exec.ParseDAGRunRef(dagRunRef)
	if err != nil {
		return fmt.Errorf("failed to parse dag-run reference %s: %w", dagRunRef, err)
	}
	return dequeueQueuedDAGRun(ctx, queueName, dagRun)
}

// dequeueFirst dequeues the first DAG run from the named queue and processes that run as aborted.
//
// It returns an error if queues are disabled, if removing an item from the queue fails,
// if the queue is empty, if retrieving the dequeued item's DAG-run data fails, or if
// processing the dequeued DAG run fails.
func dequeueFirst(ctx *Context, queueName string) error {
	// Check if queues are enabled
	if !ctx.Config.Queues.Enabled {
		return fmt.Errorf("queues are disabled in configuration")
	}
	result, err := ctx.QueueStore.ListPaginated(ctx.Context, queueName, exec.NewPaginator(1, 1))
	if err != nil {
		return fmt.Errorf("failed to list queue %s: %w", queueName, err)
	}
	if len(result.Items) == 0 {
		return fmt.Errorf("no dag-run found in queue %s", queueName)
	}

	data, err := result.Items[0].Data()
	if err != nil {
		return fmt.Errorf("failed to get dag-run data: %w", err)
	}
	return dequeueQueuedDAGRun(ctx, queueName, *data)
}

// dequeueQueuedDAGRun aborts a queued dag-run and removes its queue entries.
func dequeueQueuedDAGRun(ctx *Context, queueName string, dagRun exec.DAGRunRef) error {
	// Check if queues are enabled
	if !ctx.Config.Queues.Enabled {
		return fmt.Errorf("queues are disabled in configuration")
	}

	if err := ctx.ProcStore.Lock(ctx, queueName); err != nil {
		return fmt.Errorf("failed to lock process group %s: %w", queueName, err)
	}
	defer ctx.ProcStore.Unlock(ctx, queueName)

	if err := exec.AbortQueuedDAGRun(ctx.Context, ctx.DAGRunStore, dagRun); err != nil {
		return mapAbortQueuedDAGRunError(dagRun, err)
	}

	if _, err := ctx.QueueStore.DequeueByDAGRunID(ctx.Context, queueName, dagRun); err != nil && !errors.Is(err, exec.ErrQueueItemNotFound) {
		return fmt.Errorf("failed to dequeue dag-run %s: %w", dagRun.ID, err)
	}

	logger.Info(ctx.Context, "Dequeued dag-run",
		tag.DAG(dagRun.Name),
		tag.RunID(dagRun.ID),
		tag.Queue(queueName),
	)

	return nil
}

func mapAbortQueuedDAGRunError(dagRun exec.DAGRunRef, err error) error {
	if errors.Is(err, exec.ErrDAGRunIDNotFound) || errors.Is(err, exec.ErrNoStatusData) {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRun.ID, err)
	}

	var notQueuedErr *exec.DAGRunNotQueuedError
	if errors.As(err, &notQueuedErr) {
		if notQueuedErr.HasStatus {
			return fmt.Errorf("dag-run %s is not in queued status but %s", dagRun.ID, notQueuedErr.Status)
		}
		return fmt.Errorf("dag-run %s is not in queued status", dagRun.ID)
	}

	return err
}
