// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	rt "github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/spf13/cobra"
)

// Errors for start command
var (
	// ErrDAGRunIDRequired is returned when a sub dag-run is attempted without providing a dag-run ID
	ErrDAGRunIDRequired = errors.New("dag-run ID must be provided for sub dag-runs")
)

// Start creates and returns a cobra command for starting a dag-run
func Start() *cobra.Command {
	cmd := NewCommand(
		&cobra.Command{
			Use:   "start [flags] <DAG definition> [-- param1 param2 ...]",
			Short: "Execute a DAG from a DAG definition",
			Long: `Begin execution of a DAG-run based on the specified DAG definition.

A DAG definition is a blueprint that defines the DAG structure. This command creates a new DAG-run
instance with a unique DAG-run ID.

Parameters after the "--" separator are passed as execution parameters (either positional or key=value pairs).
Flags can override default settings such as DAG-run ID, DAG name, or suppress output.

Examples:
  dagu start my_dag -- P1=foo P2=bar
  dagu start --name my_custom_name my_dag.yaml -- P1=foo P2=bar

This command parses the DAG definition, resolves parameters, and initiates the DAG-run execution.
`,
			Args: cobra.MinimumNArgs(1),
		}, startFlags, runStart,
	)
	_ = cmd.Flags().MarkHidden("queued-run")
	return cmd
}

// Command line flags for the start command
var startFlags = []commandLineFlag{
	paramsFlag,
	nameFlag,
	dagRunIDFlag,
	fromRunIDFlag,
	parentDAGRunFlag,
	rootDAGRunFlag,
	tagsFlag,
	defaultWorkingDirFlag,
	startWorkerIDFlag,
	triggerTypeFlag,
	scheduledTimeFlag,
	queuedRunFlag,
}

var fromRunIDFlag = commandLineFlag{
	name:  "from-run-id",
	usage: "Historic dag-run ID to use as the template for a new run",
}

// startWorkerIDFlag identifies which worker executes this DAG run (for distributed execution tracking)
var startWorkerIDFlag = commandLineFlag{
	name:  "worker-id",
	usage: "Worker ID executing this DAG run (auto-set in distributed mode, defaults to 'local')",
}

// triggerTypeFlag identifies how this DAG run was initiated
var triggerTypeFlag = commandLineFlag{
	name:         "trigger-type",
	usage:        "How this DAG run was initiated (scheduler, manual, webhook, subdag, retry, catchup)",
	defaultValue: "manual",
}

var queuedRunFlag = commandLineFlag{
	name:   "queued-run",
	usage:  "Reuse an existing queued/coordinator-created attempt (internal use)",
	isBool: true,
}

func runStart(ctx *Context, args []string) error {
	fromRunID, err := ctx.StringParam("from-run-id")
	if err != nil {
		return fmt.Errorf("failed to get from-run-id: %w", err)
	}

	workerID := getWorkerID(ctx)

	triggerType, err := parseTriggerTypeParam(ctx)
	if err != nil {
		return err
	}

	scheduledTime, err := parseScheduledTimeParam(ctx, "scheduled-time")
	if err != nil {
		return err
	}

	queuedRun, err := ctx.Command.Flags().GetBool("queued-run")
	if err != nil {
		return fmt.Errorf("failed to read queued-run flag: %w", err)
	}

	dagRunID, rootRef, parentRef, isSubDAGRun, err := getDAGRunInfo(ctx)
	if err != nil {
		return err
	}

	if fromRunID != "" && isSubDAGRun {
		return fmt.Errorf("--from-run-id cannot be combined with --parent or --root")
	}

	var (
		dag    *core.DAG
		params string
	)

	if fromRunID != "" {
		if len(args) == 0 {
			return fmt.Errorf("DAG name or file must be provided when using --from-run-id")
		}
		if len(args) > 1 || ctx.Command.Flags().Changed("params") || ctx.Command.ArgsLenAtDash() != -1 {
			return fmt.Errorf("parameters cannot be provided when using --from-run-id")
		}

		dagName, err := extractDAGName(ctx, args[0])
		if err != nil {
			return fmt.Errorf("failed to resolve DAG name: %w", err)
		}

		attempt, err := ctx.DAGRunStore.FindAttempt(ctx, exec.NewDAGRunRef(dagName, fromRunID))
		if err != nil {
			return fmt.Errorf("failed to find historic dag-run %s for DAG %s: %w", fromRunID, dagName, err)
		}

		status, err := attempt.ReadStatus(ctx)
		if err != nil {
			return fmt.Errorf("failed to read status for dag-run %s: %w", fromRunID, err)
		}

		snapshot, err := attempt.ReadDAG(ctx)
		if err != nil {
			return fmt.Errorf("failed to read DAG snapshot for dag-run %s: %w", fromRunID, err)
		}

		params = status.Params
		dag, err = restoreDAGFromStatus(ctx.Context, snapshot, status)
		if err != nil {
			return fmt.Errorf("failed to restore DAG from status: %w", err)
		}

		nameOverride, err := ctx.StringParam("name")
		if err != nil {
			return fmt.Errorf("failed to read name override: %w", err)
		}
		if nameOverride != "" {
			if err := core.ValidateDAGName(nameOverride); err != nil {
				return fmt.Errorf("invalid DAG name override: %w", err)
			}
			dag.Name = nameOverride
		}
	} else {
		if err := validateStartArgumentSeparator(ctx, args); err != nil {
			return err
		}

		// Load parameters and DAG
		dag, params, err = loadDAGWithParams(ctx, args, isSubDAGRun)
		if err != nil {
			return err
		}

		if err := validateStartPositionalParamCount(ctx, args, dag); err != nil {
			return err
		}
	}

	if err := parseAndAppendTags(ctx, dag); err != nil {
		return err
	}

	root, err := determineRootDAGRun(isSubDAGRun, rootRef, dag, dagRunID)
	if err != nil {
		return err
	}

	ctx.Context = logger.WithValues(ctx.Context, tag.DAG(dag.Name), tag.RunID(dagRunID))

	statusSeed, err := loadStartStatusSeed(ctx, dag.Name, dagRunID, root, isSubDAGRun, workerID, queuedRun)
	if err != nil {
		return err
	}

	if statusSeed != nil && statusSeed.Status == core.Queued && !queuedRun && workerID == "local" {
		return fmt.Errorf("dag-run ID %s already exists for DAG %s (status: %s)", dagRunID, dag.Name, statusSeed.Status)
	}
	if statusSeed != nil && statusSeed.Status != core.NotStarted && statusSeed.Status != core.Queued {
		return fmt.Errorf("dag-run ID %s already exists for DAG %s (status: %s)", dagRunID, dag.Name, statusSeed.Status)
	}

	if fromRunID != "" {
		logger.Info(ctx, "Rescheduling dag-run",
			slog.String("from-dag-run-id", fromRunID),
			slog.String("params", params),
		)
	} else {
		logger.Info(ctx, "Executing root dag-run", slog.String("params", params))
	}

	req := newStartExecutionRequest(
		dag,
		dagRunID,
		root,
		workerID,
		triggerType,
		scheduledTime,
		queuedRun,
		statusSeed,
	)

	if isSubDAGRun {
		parent, err := exec.ParseDAGRunRef(parentRef)
		if err != nil {
			return fmt.Errorf("failed to parse parent dag-run reference: %w", err)
		}
		return handleSubDAGRun(ctx, req, params, parent)
	}

	return tryExecuteDAG(ctx, req)
}

var errProcAcquisitionFailed = errors.New("failed to acquire process handle")

type startExecutionRequest struct {
	dag      *core.DAG
	dagRunID string
	root     exec.DAGRunRef
	workerID string
	metadata rt.StartMetadata
}

// tryExecuteDAG acquires a process handle and executes the DAG.
func tryExecuteDAG(ctx *Context, req startExecutionRequest) error {
	// Check for dispatch to coordinator for distributed execution.
	// Skip if already running on a worker (workerID != "local").
	if req.workerID == "local" {
		coordinatorCli := ctx.NewCoordinatorClient()
		if core.ShouldDispatchToCoordinator(req.dag, coordinatorCli != nil, ctx.Config.DefaultExecMode) {
			return dispatchToCoordinatorAndWait(ctx, req, coordinatorCli)
		}
	}

	if err := ctx.ProcStore.Lock(ctx, req.dag.ProcGroup()); err != nil {
		logger.Debug(ctx, "Failed to lock process group", tag.Error(err))
		_ = ctx.RecordEarlyFailure(req.dag, req.dagRunID, err)
		return errProcAcquisitionFailed
	}

	proc, err := ctx.ProcStore.Acquire(ctx, req.dag.ProcGroup(), exec.NewDAGRunRef(req.dag.Name, req.dagRunID))
	if err != nil {
		ctx.ProcStore.Unlock(ctx, req.dag.ProcGroup())
		logger.Debug(ctx, "Failed to acquire process handle", tag.Error(err))
		_ = ctx.RecordEarlyFailure(req.dag, req.dagRunID, err)
		return fmt.Errorf("failed to acquire process handle: %w", errProcAcquisitionFailed)
	}
	defer func() {
		_ = proc.Stop(ctx)
	}()
	ctx.Proc = proc

	ctx.ProcStore.Unlock(ctx, req.dag.ProcGroup())

	return executeDAGRun(ctx, req, exec.DAGRunRef{})
}

// getDAGRunInfo extracts and validates dag-run ID and references from command flags.
// nolint:revive
func getDAGRunInfo(ctx *Context) (dagRunID, rootDAGRun, parentDAGRun string, isSubDAGRun bool, err error) {
	dagRunID, err = ctx.StringParam("run-id")
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	rootDAGRun, _ = ctx.Command.Flags().GetString("root")
	parentDAGRun, _ = ctx.Command.Flags().GetString("parent")
	isSubDAGRun = parentDAGRun != ""

	if isSubDAGRun && dagRunID == "" {
		return "", "", "", false, ErrDAGRunIDRequired
	}

	if dagRunID != "" {
		if err := validateRunID(dagRunID); err != nil {
			return "", "", "", false, err
		}
	} else {
		dagRunID, err = genRunID()
		if err != nil {
			return "", "", "", false, fmt.Errorf("failed to generate dag-run ID: %w", err)
		}
	}

	return dagRunID, rootDAGRun, parentDAGRun, isSubDAGRun, nil
}

// loadDAGWithParams loads the DAG and its parameters from command arguments.
func loadDAGWithParams(ctx *Context, args []string, isSubDAGRun bool) (*core.DAG, string, error) {
	dagPath := args[0]

	loadOpts := []spec.LoadOption{
		spec.WithBaseConfig(ctx.Config.Paths.BaseConfig),
		spec.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	}

	if isSubDAGRun {
		loadOpts = append(loadOpts, spec.WithSkipBaseHandlers())
	}

	nameOverride, err := ctx.StringParam("name")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get name override: %w", err)
	}
	if nameOverride != "" {
		loadOpts = append(loadOpts, spec.WithName(nameOverride))
	}

	defaultWorkingDir, err := ctx.StringParam("default-working-dir")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get default-working-dir: %w", err)
	}
	if defaultWorkingDir != "" {
		loadOpts = append(loadOpts, spec.WithDefaultWorkingDir(defaultWorkingDir))
	}

	var params string

	if ctx.Command.ArgsLenAtDash() != -1 && len(args) > 0 {
		loadOpts = append(loadOpts, spec.WithParams(args[ctx.Command.ArgsLenAtDash():]))
	} else {
		params, err = ctx.Command.Flags().GetString("params")
		if err != nil {
			return nil, "", fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, spec.WithParams(stringutil.RemoveQuotes(params)))
	}

	dag, err := spec.Load(ctx, dagPath, loadOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load DAG from %s: %w", dagPath, err)
	}

	return dag, params, nil
}

// parseAndAppendTags parses the --tags flag and appends validated tags to the DAG.
func parseAndAppendTags(ctx *Context, dag *core.DAG) error {
	tagsStr, err := ctx.StringParam("tags")
	if err != nil {
		return fmt.Errorf("failed to get tags: %w", err)
	}
	if tagsStr != "" {
		extraTags := core.NewTags(strings.Split(tagsStr, ","))
		if err := core.ValidateTags(extraTags); err != nil {
			return fmt.Errorf("invalid tags: %w", err)
		}
		dag.Tags = append(dag.Tags, extraTags...)
	}
	return nil
}

// determineRootDAGRun creates or parses the root execution reference.
func determineRootDAGRun(isSubDAGRun bool, rootDAGRun string, dag *core.DAG, dagRunID string) (exec.DAGRunRef, error) {
	if isSubDAGRun {
		ref, err := exec.ParseDAGRunRef(rootDAGRun)
		if err != nil {
			return exec.DAGRunRef{}, fmt.Errorf("failed to parse root exec ref: %w", err)
		}
		return ref, nil
	}
	return exec.NewDAGRunRef(dag.Name, dagRunID), nil
}

func loadStartStatusSeed(
	ctx *Context,
	dagName string,
	dagRunID string,
	root exec.DAGRunRef,
	isSubDAGRun bool,
	workerID string,
	queuedRun bool,
) (*exec.DAGRunStatus, error) {
	ref := exec.NewDAGRunRef(dagName, dagRunID)

	var (
		attempt exec.DAGRunAttempt
		err     error
	)
	if isSubDAGRun {
		attempt, err = ctx.DAGRunStore.FindSubAttempt(ctx, root, dagRunID)
	} else {
		attempt, err = ctx.DAGRunStore.FindAttempt(ctx, ref)
	}
	if errors.Is(err, exec.ErrDAGRunIDNotFound) {
		if queuedRun {
			return nil, fmt.Errorf("queued start requires an existing dag-run ID %s for DAG %s", dagRunID, dagName)
		}
		if workerID != "local" {
			return nil, fmt.Errorf("worker start requires an existing dag-run ID %s for DAG %s", dagRunID, dagName)
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load existing dag-run %s: %w", ref.String(), err)
	}

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read existing status for dag-run %s: %w", ref.String(), err)
	}
	if queuedRun && status.Status != core.Queued {
		return nil, fmt.Errorf("queued start requires dag-run ID %s for DAG %s to be queued (status: %s)", dagRunID, dagName, status.Status)
	}

	return status, nil
}

func newStartExecutionRequest(
	dag *core.DAG,
	dagRunID string,
	root exec.DAGRunRef,
	workerID string,
	triggerType core.TriggerType,
	scheduledTime string,
	reuseAttempt bool,
	statusSeed *exec.DAGRunStatus,
) startExecutionRequest {
	return startExecutionRequest{
		dag:      dag,
		dagRunID: dagRunID,
		root:     root,
		workerID: workerID,
		metadata: rt.StartMetadata{
			TriggerType:   triggerType,
			ScheduledTime: scheduledTime,
			StatusSeed:    statusSeed,
			ReuseAttempt:  workerID != "local" || reuseAttempt,
		}.Normalized(),
	}
}

// handleSubDAGRun processes a sub dag-run, checking for previous runs.
func handleSubDAGRun(ctx *Context, req startExecutionRequest, params string, parent exec.DAGRunRef) error {
	logger.Info(ctx, "Executing sub dag-run",
		slog.String("params", params),
		slog.Any("root", req.root),
		slog.Any("parent", parent),
		slog.String("workerID", req.workerID),
	)

	if req.dagRunID == "" {
		return fmt.Errorf("dag-run ID must be provided for sub DAGrun")
	}

	// For distributed execution, the coordinator already created the sub-attempt record.
	if req.workerID != "local" {
		return executeDAGRun(ctx, req, parent)
	}

	logger.Debug(ctx, "Checking for previous sub dag-run with the dag-run ID")

	subAttempt, err := ctx.DAGRunStore.FindSubAttempt(ctx, req.root, req.dagRunID)
	if errors.Is(err, exec.ErrDAGRunIDNotFound) {
		return executeDAGRun(ctx, req, parent)
	}
	if err != nil {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", req.dagRunID, err)
	}

	status, err := subAttempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read previous run status for dag-run ID %s: %w", req.dagRunID, err)
	}

	return executeRetry(ctx, req.dag, status, req.root, "", req.workerID)
}

// executeDAGRun initializes execution state for a DAG run and invokes the shared agent executor.
func executeDAGRun(ctx *Context, req startExecutionRequest, parent exec.DAGRunRef) error {
	logFile, err := ctx.OpenLogFile(req.dag, req.dagRunID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", req.dag.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Debug(ctx, "Dag-run initiated", tag.File(logFile.Name()))

	dr, err := ctx.dagStore(dagStoreConfig{
		SearchPaths:           []string{filepath.Dir(req.dag.Location)},
		SkipDirectoryCreation: req.workerID != "local",
	})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	as := ctx.agentStores()

	agentInstance := agent.New(
		req.dagRunID,
		req.dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.DAGRunMgr,
		dr,
		agent.Options{
			ParentDAGRun:            parent,
			ProgressDisplay:         shouldEnableProgress(ctx),
			WorkerID:                req.workerID,
			QueuedRun:               req.metadata.ReuseAttempt,
			DAGRunStore:             ctx.DAGRunStore,
			ServiceRegistry:         ctx.ServiceRegistry,
			RootDAGRun:              req.root,
			PeerConfig:              ctx.Config.Core.Peer,
			TriggerType:             req.metadata.TriggerType,
			ScheduledTime:           req.metadata.ScheduledTime,
			StatusSeed:              req.metadata.StatusSeed,
			DefaultExecMode:         ctx.Config.DefaultExecMode,
			AgentConfigStore:        as.ConfigStore,
			AgentModelStore:         as.ModelStore,
			AgentMemoryStore:        as.MemoryStore,
			AgentSkillStore:         as.SkillStore,
			AgentSoulStore:          as.SoulStore,
			AgentRemoteNodeResolver: as.RemoteNodeResolver,
		},
	)

	return ExecuteAgent(ctx, agentInstance, req.dag, req.dagRunID, logFile)
}

// dispatchToCoordinatorAndWait dispatches a DAG to coordinator and waits for completion.
func dispatchToCoordinatorAndWait(ctx *Context, req startExecutionRequest, coordinatorCli coordinator.Client) error {
	signalCtx, stop := signal.NotifyContext(ctx.Context, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	signalAwareCtx := ctx.WithContext(signalCtx)

	showProgress := shouldEnableProgress(ctx)
	var progress *RemoteProgressDisplay
	if showProgress {
		progress = NewRemoteProgressDisplay(req.dag, req.dagRunID)
		progress.Start()
	}

	defer func() {
		if progress != nil {
			progress.Stop()
			if !ctx.Quiet {
				progress.PrintSummary()
			}
		}
	}()

	logger.Info(ctx, "Dispatching DAG for distributed execution",
		slog.Any("worker-selector", req.dag.WorkerSelector),
	)

	var taskOpts []executor.TaskOption
	if len(req.dag.WorkerSelector) > 0 {
		taskOpts = append(taskOpts, executor.WithWorkerSelector(req.dag.WorkerSelector))
	}
	if len(req.dag.Tags) > 0 {
		taskOpts = append(taskOpts, executor.WithTags(strings.Join(req.dag.Tags.Strings(), ",")))
	}
	if req.metadata.TriggerType != core.TriggerTypeUnknown {
		taskOpts = append(taskOpts, executor.WithTaskTriggerType(req.metadata.TriggerType.String()))
	}
	if req.metadata.ScheduledTime != "" {
		taskOpts = append(taskOpts, executor.WithTaskScheduledTime(req.metadata.ScheduledTime))
	}
	if req.metadata.StatusSeed != nil {
		taskOpts = append(taskOpts, executor.WithPreviousStatus(req.metadata.StatusSeed))
	}

	task := executor.CreateTask(
		req.dag.Name,
		string(req.dag.YamlData),
		coordinatorv1.Operation_OPERATION_START,
		req.dagRunID,
		taskOpts...,
	)

	if err := coordinatorCli.Dispatch(signalAwareCtx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	logger.Info(ctx, "DAG dispatched to coordinator; awaiting completion")
	err := waitForDAGCompletionWithProgress(signalAwareCtx, req.dag, req.dagRunID, coordinatorCli, progress)

	// If context was cancelled (e.g., Ctrl+C), request cancellation on coordinator
	if signalCtx.Err() != nil {
		return handleDistributedCancellation(ctx, req.dag, req.dagRunID, coordinatorCli, progress, err)
	}

	return err
}

// handleDistributedCancellation handles the cancellation of a distributed DAG run when a signal is received.
// It requests cancellation from the coordinator and polls for status updates until the DAG is no longer active.
func handleDistributedCancellation(ctx context.Context, dag *core.DAG, dagRunID string, coordinatorCli coordinator.Client, progress *RemoteProgressDisplay, originalErr error) error {
	logger.Info(ctx, "Requesting cancellation of distributed DAG run", tag.RunID(dagRunID))
	cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if cancelErr := coordinatorCli.RequestCancel(cancelCtx, dag.Name, dagRunID, nil); cancelErr != nil {
		logger.Warn(ctx, "Failed to request cancellation", tag.Error(cancelErr))
	}

	if progress == nil {
		return originalErr
	}

	// Poll for up to 5 seconds until status reflects cancellation
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cancelCtx.Done():
			progress.SetCancelled()
			return originalErr
		case <-ticker.C:
			resp, fetchErr := coordinatorCli.GetDAGRunStatus(cancelCtx, dag.Name, dagRunID, nil)
			if fetchErr != nil || resp == nil || resp.Status == nil {
				continue
			}
			progress.Update(resp.Status)
			dagStatus, convErr := convert.ProtoToDAGRunStatus(resp.Status)
			if convErr == nil && dagStatus != nil && !dagStatus.Status.IsActive() {
				return originalErr
			}
		}
	}
}

// waitForDAGCompletionWithProgress polls the coordinator until the DAG run completes.
// Progress display is managed by the caller.
func waitForDAGCompletionWithProgress(ctx *Context, d *core.DAG, dagRunID string, coordinatorCli coordinator.Client, progress *RemoteProgressDisplay) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	logTicker := time.NewTicker(15 * time.Second)
	defer logTicker.Stop()

	var consecutiveErrors int
	const maxConsecutiveErrors = 10 // Fail after 10 consecutive errors (10 seconds)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-logTicker.C:
			if progress == nil {
				// Only log if not showing progress (progress display shows its own updates)
				logger.Info(ctx, "Waiting for DAG completion...", tag.RunID(dagRunID))
			}

		case <-ticker.C:
			resp, err := coordinatorCli.GetDAGRunStatus(ctx, d.Name, dagRunID, nil)
			if err != nil {
				consecutiveErrors++
				logger.Debug(ctx, "Failed to get status from coordinator",
					tag.Error(err), slog.Int("consecutive_errors", consecutiveErrors))

				if consecutiveErrors >= maxConsecutiveErrors {
					return fmt.Errorf("lost connection to coordinator after %d attempts: %w", consecutiveErrors, err)
				}
				continue
			}
			consecutiveErrors = 0 // Reset on success

			if resp == nil || resp.Status == nil {
				continue
			}

			// Update progress display with current status
			if progress != nil {
				progress.Update(resp.Status)
			}

			// Check status
			dagStatus, convErr := convert.ProtoToDAGRunStatus(resp.Status)
			if convErr != nil || dagStatus == nil {
				continue
			}
			if !dagStatus.Status.IsActive() {
				if dagStatus.Status.IsSuccess() {
					logger.Info(ctx, "DAG completed successfully", tag.RunID(dagRunID))
					return nil
				}
				// Include error details from response if available
				if resp.Error != "" {
					return fmt.Errorf("DAG run failed with status %s: %s", dagStatus.Status, resp.Error)
				}
				return fmt.Errorf("DAG run failed with status: %s", dagStatus.Status)
			}
		}
	}
}
