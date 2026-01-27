package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/spf13/cobra"
)

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
		}, enqueueFlags, runEnqueue,
	)
}

var enqueueFlags = []commandLineFlag{paramsFlag, nameFlag, dagRunIDFlag, queueFlag, defaultWorkingDirFlag, triggerTypeFlag}

func runEnqueue(ctx *Context, args []string) error {
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

	triggerTypeStr, _ := ctx.StringParam("trigger-type")
	triggerType := core.ParseTriggerType(triggerTypeStr)

	return enqueueDAGRun(ctx, dag, runID, triggerType)
}

// enqueueDAGRun enqueues a dag-run to the queue.
// The DAG location is cleared to allow concurrent queued runs (location is used
// for unix pipe generation which would prevent parallel execution).
func enqueueDAGRun(ctx *Context, dag *core.DAG, dagRunID string, triggerType core.TriggerType) error {
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

	att, err := ctx.DAGRunStore.CreateAttempt(ctx.Context, dag, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}

	opts := []transform.StatusOption{
		transform.WithLogFilePath(logFile),
		transform.WithAttemptID(att.ID()),
		transform.WithPreconditions(dag.Preconditions),
		transform.WithQueuedAt(stringutil.FormatTime(time.Now())),
		transform.WithHierarchyRefs(
			exec.NewDAGRunRef(dag.Name, dagRunID),
			exec.DAGRunRef{},
		),
		transform.WithTriggerType(triggerType),
	}

	dagStatus := transform.NewStatusBuilder(dag).Create(dagRunID, core.Queued, 0, time.Time{}, opts...)

	if err := att.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		_ = att.Close(ctx.Context)
	}()

	if err := att.Write(ctx.Context, dagStatus); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	if err := ctx.QueueStore.Enqueue(ctx.Context, dag.ProcGroup(), exec.QueuePriorityLow, dagRun); err != nil {
		return fmt.Errorf("failed to enqueue dag-run: %w", err)
	}

	logger.Info(ctx.Context, "Enqueued dag-run",
		tag.DAG(dag.Name),
		tag.RunID(dagRunID),
		slog.Any("params", dag.Params),
	)

	return nil
}
