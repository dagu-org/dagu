package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmd/dagpicker"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/builder"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dagu-org/dagu/internal/stringutil"
)

// Errors for start command
var (
	// ErrDAGRunIDRequired is returned when a child dag-run is attempted without providing a dag-run ID
	ErrDAGRunIDRequired = errors.New("dag-run ID must be provided for child dag-runs")

	// ErrDAGRunIDFormat is returned when the provided dag-run ID is not valid
	ErrDAGRunIDFormat = errors.New("dag-run ID must only contain alphanumeric characters, dashes, and underscores")

	// ErrDAGRunIDTooLong is returned when the provided dag-run ID is too long
	ErrDAGRunIDTooLong = errors.New("dag-run ID length must be less than 64 characters")
)

// CmdStart creates and returns a cobra command for starting a dag-run
func CmdStart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "start [flags] <DAG definition> [-- param1 param2 ...]",
			Short: "Execute a DAG from a DAG definition",
			Long: `Begin execution of a DAG-run based on the specified DAG definition.

A DAG definition is a blueprint that defines the DAG structure. This command creates a new DAG-run 
instance with a unique DAG-run ID.

Parameters after the "--" separator are passed as execution parameters (either positional or key=value pairs).
Flags can override default settings such as DAG-run ID or suppress output.

Example:
  dagu start my_dag -- P1=foo P2=bar

This command parses the DAG definition, resolves parameters, and initiates the DAG-run execution.
`,
			Args: cobra.ArbitraryArgs,
		}, startFlags, runStart,
	)
}

// Command line flags for the start command
var startFlags = []commandLineFlag{paramsFlag, dagRunIDFlag, parentDAGRunFlag, rootDAGRunFlag, noQueueFlag}

// runStart handles the execution of the start command
func runStart(ctx *Context, args []string) error {
	// Get dag-run ID and references
	dagRunID, rootRef, parentRef, isChildDAGRun, err := getDAGRunInfo(ctx)
	if err != nil {
		return err
	}

	// Load parameters and DAG
	dag, params, err := loadDAGWithParams(ctx, args)
	if err != nil {
		return err
	}

	// Create or get root execution reference
	root, err := determineRootDAGRun(isChildDAGRun, rootRef, dag, dagRunID)
	if err != nil {
		return err
	}

	// Handle child dag-run if applicable
	if isChildDAGRun {
		// Parse parent execution reference
		parent, err := digraph.ParseDAGRunRef(parentRef)
		if err != nil {
			return fmt.Errorf("failed to parse parent dag-run reference: %w", err)
		}
		return handleChildDAGRun(ctx, dag, dagRunID, params, root, parent)
	}

	var queueDisabled bool
	if os.Getenv("DISABLE_DAG_RUN_QUEUE") != "" {
		queueDisabled = true
	}

	// check no-queue flag
	if ctx.Command.Flags().Changed("no-queue") {
		queueDisabled = true
	}

	// Check if the DAG run-id is unique
	attempt, _ := ctx.DAGRunStore.FindAttempt(ctx, root)
	if attempt != nil {
		// If the dag-run ID already exists, we cannot start a new run with the same ID
		return fmt.Errorf("dag-run ID %s already exists for DAG %s", dagRunID, dag.Name)
	}

	// Count running DAG to check against maxActiveRuns setting (best effort).
	liveCount, err := ctx.ProcStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return fmt.Errorf("failed to access proc store: %w", err)
	}
	if dag.MaxActiveRuns == 1 && liveCount > 0 {
		return fmt.Errorf("DAG %s is already running, cannot start", dag.Name)
	}

	// Log root dag-run
	logger.Info(ctx, "Executing root dag-run",
		"dag", dag.Name,
		"params", params,
		"dagRunId", dagRunID,
	)

	err = tryExecuteDAG(ctx, dag, dagRunID, root)
	if errors.Is(err, errMaxRunReached) && !queueDisabled {
		dag.Location = "" // Queued dag-runs must not have a location

		// If the DAG has a queue configured and maxActiveRuns > 1, ensure the number
		// of active runs in the queue does not exceed this limit.
		// The scheduler only enforces maxActiveRuns at the global queue level.
		queuedRuns, err := ctx.QueueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
		if err != nil {
			return fmt.Errorf("failed to read queue: %w", err)
		}
		if dag.Queue != "" && dag.MaxActiveRuns > 1 && len(queuedRuns)+liveCount >= dag.MaxActiveRuns {
			return fmt.Errorf("DAG %s is already in the queue (maxActiveRuns=%d), cannot start", dag.Name, dag.MaxActiveRuns)
		}

		// Enqueue the DAG-run for execution
		return enqueueDAGRun(ctx, dag, dagRunID)
	}

	return err // return executed result
}

var (
	errMaxRunReached = errors.New("max run reached")
)

// tryExecuteDAG tries to run the DAG within the max concurrent run config
func tryExecuteDAG(ctx *Context, dag *digraph.DAG, dagRunID string, root digraph.DAGRunRef) error {
	if err := ctx.ProcStore.TryLock(ctx, dag.ProcGroup()); err != nil {
		logger.Debug(ctx, "failed to lock process group", "err", err)
		return errMaxRunReached
	}
	defer ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	runningCount, err := ctx.ProcStore.CountAlive(ctx, dag.ProcGroup())
	if err != nil {
		logger.Debug(ctx, "failed to count live processes", "err", err)
		return fmt.Errorf("failed to count live process for %s: %w", dag.ProcGroup(), errMaxRunReached)
	}

	// If the DAG has a queue configured and maxActiveRuns > 0, ensure the number
	// of active runs in the queue does not exceed this limit.
	if dag.MaxActiveRuns > 0 && runningCount >= dag.MaxActiveRuns {
		// It's not possible to run right now.
		return fmt.Errorf("max active run is reached (%d >= %d): %w", runningCount, dag.MaxActiveRuns, errMaxRunReached)
	}

	// Acquire process handle
	proc, err := ctx.ProcStore.Acquire(ctx, dag.ProcGroup(), digraph.NewDAGRunRef(dag.Name, dagRunID))
	if err != nil {
		logger.Debug(ctx, "failed to acquire process handle", "err", err)
		return fmt.Errorf("failed to acquire process handle: %w", errMaxRunReached)
	}
	defer func() {
		_ = proc.Stop(ctx)
	}()
	ctx.Proc = proc

	// Unlock the process group
	ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	return executeDAGRun(ctx, dag, digraph.DAGRunRef{}, dagRunID, root)
}

// getDAGRunInfo extracts and validates dag-run ID and references from command flags
// nolint:revive
func getDAGRunInfo(ctx *Context) (dagRunID, rootDAGRun, parentDAGRun string, isChildDAGRun bool, err error) {
	dagRunID, err = ctx.StringParam("run-id")
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	// Get root and parent execution references
	rootDAGRun, _ = ctx.Command.Flags().GetString("root")
	parentDAGRun, _ = ctx.Command.Flags().GetString("parent")
	isChildDAGRun = parentDAGRun != "" || rootDAGRun != ""

	// Validate dag-run ID for child dag-runs
	if isChildDAGRun && dagRunID == "" {
		return "", "", "", false, ErrDAGRunIDRequired
	}

	// Validate or generate dag-run ID
	if dagRunID != "" {
		if err := validateRunID(dagRunID); err != nil {
			return "", "", "", false, err
		}
	} else {
		// Generate a new dag-run ID if not provided
		dagRunID, err = genRunID()
		if err != nil {
			return "", "", "", false, fmt.Errorf("failed to generate dag-run ID: %w", err)
		}
	}

	return dagRunID, rootDAGRun, parentDAGRun, isChildDAGRun, nil
}

// loadDAGWithParams loads the DAG and its parameters from command arguments
func loadDAGWithParams(ctx *Context, args []string) (*digraph.DAG, string, error) {
	var dagPath string
	var interactiveParams string

	// Check if DAG path is provided
	if len(args) == 0 {
		// Check if we're in an interactive terminal
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, "", fmt.Errorf("DAG file path is required")
		}

		// Use interactive picker
		logger.Info(ctx, "No DAG specified, opening interactive selector...")

		// Get DAG store
		dagStore, err := ctx.dagStore(nil, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to initialize DAG store: %w", err)
		}

		// Load DAG metadata first to pass to the picker
		// This will be updated when user selects a DAG
		var tempDAG *digraph.DAG

		// Show unified interactive UI
		result, err := dagpicker.PickDAGInteractive(ctx, dagStore, tempDAG)
		if err != nil {
			return nil, "", err
		}

		if result.Cancelled {
			fmt.Println("DAG execution cancelled.")
			os.Exit(0)
		}

		dagPath = result.DAGPath
		interactiveParams = result.Params
	} else {
		dagPath = args[0]
	}

	// Prepare load options with base configuration
	loadOpts := []builder.LoadOption{
		builder.WithBaseConfig(ctx.Config.Paths.BaseConfig),
		builder.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	}

	// Load parameters from command line arguments
	var params string
	var err error

	// Check if parameters are provided after "--"
	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 && len(args) > 0 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, builder.WithParams(args[argsLenAtDash:]))
	} else if interactiveParams != "" {
		// Use interactive parameters
		loadOpts = append(loadOpts, builder.WithParams(stringutil.RemoveQuotes(interactiveParams)))
		params = interactiveParams
	} else {
		// Get parameters from flags
		params, err = ctx.Command.Flags().GetString("params")
		if err != nil {
			return nil, "", fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, builder.WithParams(stringutil.RemoveQuotes(params)))
	}

	// Load the DAG from the specified file
	dag, err := builder.Load(ctx, dagPath, loadOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load DAG from %s: %w", dagPath, err)
	}

	return dag, params, nil
}

// determineRootDAGRun creates or parses the root execution reference
func determineRootDAGRun(isChildDAGRun bool, rootDAGRun string, dag *digraph.DAG, dagRunID string) (digraph.DAGRunRef, error) {
	if isChildDAGRun {
		// Parse the rootDAGRun execution reference for child dag-runs
		rootDAGRun, err := digraph.ParseDAGRunRef(rootDAGRun)
		if err != nil {
			return digraph.DAGRunRef{}, fmt.Errorf("failed to parse root exec ref: %w", err)
		}
		return rootDAGRun, nil
	}

	// Create a new root execution reference for root execution
	return digraph.NewDAGRunRef(dag.Name, dagRunID), nil
}

// handleChildDAGRun processes a child dag-run, checking for previous runs
func handleChildDAGRun(ctx *Context, dag *digraph.DAG, dagRunID string, params string, root digraph.DAGRunRef, parent digraph.DAGRunRef) error {
	// Log child dag-run execution
	logger.Info(ctx, "Executing child dag-run",
		"dag", dag.Name,
		"params", params,
		"dagRunId", dagRunID,
		"root", root,
		"parent", parent,
	)

	// Double-check dag-run ID is provided (should be caught earlier, but being defensive)
	if dagRunID == "" {
		return fmt.Errorf("dag-run ID must be provided for child DAGrun")
	}

	// Check for previous child dag-run with this ID
	logger.Debug(ctx, "Checking for previous child dag-run with the dag-run ID", "dagRunId", dagRunID)

	// Look for existing execution childAttempt
	childAttempt, err := ctx.DAGRunStore.FindChildAttempt(ctx, root, dagRunID)
	if errors.Is(err, models.ErrDAGRunIDNotFound) {
		// If the dag-run ID is not found, proceed with new execution
		return executeDAGRun(ctx, dag, parent, dagRunID, root)
	}
	if err != nil {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRunID, err)
	}

	// Read the status of the previous run
	status, err := childAttempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read previous run status for dag-run ID %s: %w", dagRunID, err)
	}

	// Execute as a retry of the previous run
	return executeRetry(ctx, dag, status, root, "")
}

// executeDAGRun handles the actual execution of a DAG
func executeDAGRun(ctx *Context, d *digraph.DAG, parent digraph.DAGRunRef, dagRunID string, root digraph.DAGRunRef) error {
	// Open the log file for the scheduler. The log file will be used for future
	// execution for the same DAG/dag-run ID between attempts.
	logFile, err := ctx.OpenLogFile(d, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", d.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Debug(ctx, "dag-run initiated", "DAG", d.Name, "dagRunId", dagRunID, "logFile", logFile.Name())

	// Initialize DAG repository with the DAG's directory in the search path
	dr, err := ctx.dagStore(nil, []string{filepath.Dir(d.Location)})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	// Create a new agent to execute the DAG
	agentInstance := agent.New(
		dagRunID,
		d,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.DAGRunMgr,
		dr,
		ctx.DAGRunStore,
		ctx.ServiceRegistry,
		root,
		ctx.Config.Global.Peer,
		agent.Options{
			ParentDAGRun:    parent,
			ProgressDisplay: shouldEnableProgress(ctx),
		},
	)

	// Use the shared agent execution function
	return ExecuteAgent(ctx, agentInstance, d, dagRunID, logFile)
}
