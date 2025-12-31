package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmd/dagpicker"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// Errors for start command
var (
	// ErrDAGRunIDRequired is returned when a sub dag-run is attempted without providing a dag-run ID
	ErrDAGRunIDRequired = errors.New("dag-run ID must be provided for sub dag-runs")

	// ErrDAGRunIDFormat is returned when the provided dag-run ID is not valid
	ErrDAGRunIDFormat = errors.New("dag-run ID must only contain alphanumeric characters, dashes, and underscores")

	// ErrDAGRunIDTooLong is returned when the provided dag-run ID is too long
	ErrDAGRunIDTooLong = errors.New("dag-run ID length must be less than 64 characters")
)

// Start creates and returns a cobra command for starting a dag-run
func Start() *cobra.Command {
	return NewCommand(
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
			Args: cobra.ArbitraryArgs,
		}, startFlags, runStart,
	)
}

// Command line flags for the start command
var startFlags = []commandLineFlag{paramsFlag, nameFlag, dagRunIDFlag, fromRunIDFlag, parentDAGRunFlag, rootDAGRunFlag, defaultWorkingDirFlag, startWorkerIDFlag}

var fromRunIDFlag = commandLineFlag{
	name:  "from-run-id",
	usage: "Historic dag-run ID to use as the template for a new run",
}

// startWorkerIDFlag identifies which worker executes this DAG run (for distributed execution tracking)
var startWorkerIDFlag = commandLineFlag{
	name:  "worker-id",
	usage: "Worker ID executing this DAG run (auto-set in distributed mode, defaults to 'local')",
}

// runStart handles the execution of the start command
func runStart(ctx *Context, args []string) error {
	fromRunID, err := ctx.StringParam("from-run-id")
	if err != nil {
		return fmt.Errorf("failed to get from-run-id: %w", err)
	}

	// Get worker-id for tracking which worker executes this DAG run
	// Default to "local" for non-distributed execution or if flag is not defined
	workerID, _ := ctx.StringParam("worker-id")
	if workerID == "" {
		workerID = "local"
	}

	// Get dag-run ID and references
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

		attempt, err := ctx.DAGRunStore.FindAttempt(ctx, execution.NewDAGRunRef(dagName, fromRunID))
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

		dag = snapshot
		params = status.Params
		dag.Params = status.ParamsList

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
		// Load parameters and DAG
		dag, params, err = loadDAGWithParams(ctx, args, isSubDAGRun)
		if err != nil {
			return err
		}
	}

	// Create or get root execution reference
	root, err := determineRootDAGRun(isSubDAGRun, rootRef, dag, dagRunID)
	if err != nil {
		return err
	}

	// Set DAG context for all logs
	ctx.Context = logger.WithValues(ctx.Context, tag.DAG(dag.Name), tag.RunID(dagRunID))

	// Handle sub dag-run if applicable
	if isSubDAGRun {
		// Parse parent execution reference
		parent, err := execution.ParseDAGRunRef(parentRef)
		if err != nil {
			return fmt.Errorf("failed to parse parent dag-run reference: %w", err)
		}
		return handleSubDAGRun(ctx, dag, dagRunID, params, root, parent, workerID)
	}

	// Check if the DAG run-id is unique
	attempt, _ := ctx.DAGRunStore.FindAttempt(ctx, root)
	if attempt != nil {
		// If the dag-run ID already exists, we cannot start a new run with the same ID
		return fmt.Errorf("dag-run ID %s already exists for DAG %s", dagRunID, dag.Name)
	}

	// Log root dag-run or reschedule action
	if fromRunID != "" {
		logger.Info(ctx, "Rescheduling dag-run",
			slog.String("from-dag-run-id", fromRunID),
			slog.String("params", params),
		)
	} else {
		logger.Info(ctx, "Executing root dag-run", slog.String("params", params))
	}

	return tryExecuteDAG(ctx, dag, dagRunID, root, workerID)
}

var (
	errProcAcquisitionFailed = errors.New("failed to acquire process handle")
)

// tryExecuteDAG tries to run the DAG.
func tryExecuteDAG(ctx *Context, dag *core.DAG, dagRunID string, root execution.DAGRunRef, workerID string) error {
	if err := ctx.ProcStore.Lock(ctx, dag.ProcGroup()); err != nil {
		logger.Debug(ctx, "Failed to lock process group", tag.Error(err))
		_ = ctx.RecordEarlyFailure(dag, dagRunID, err)
		return errProcAcquisitionFailed
	}

	// Acquire process handle
	proc, err := ctx.ProcStore.Acquire(ctx, dag.ProcGroup(), execution.NewDAGRunRef(dag.Name, dagRunID))
	if err != nil {
		ctx.ProcStore.Unlock(ctx, dag.ProcGroup())
		logger.Debug(ctx, "Failed to acquire process handle", tag.Error(err))
		_ = ctx.RecordEarlyFailure(dag, dagRunID, err)
		return fmt.Errorf("failed to acquire process handle: %w", errProcAcquisitionFailed)
	}
	defer func() {
		_ = proc.Stop(ctx)
	}()
	ctx.Proc = proc

	// Unlock the process group immediately after acquiring the handle
	// to allow other instances of the same DAG to start.
	ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	return executeDAGRun(ctx, dag, execution.DAGRunRef{}, dagRunID, root, workerID)
}

// getDAGRunInfo extracts and validates dag-run ID and references from command flags
// nolint:revive
func getDAGRunInfo(ctx *Context) (dagRunID, rootDAGRun, parentDAGRun string, isSubDAGRun bool, err error) {
	dagRunID, err = ctx.StringParam("run-id")
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	// Get root and parent execution references
	rootDAGRun, _ = ctx.Command.Flags().GetString("root")
	parentDAGRun, _ = ctx.Command.Flags().GetString("parent")
	isSubDAGRun = parentDAGRun != "" || rootDAGRun != ""

	// Validate dag-run ID for sub dag-runs
	if isSubDAGRun && dagRunID == "" {
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

	return dagRunID, rootDAGRun, parentDAGRun, isSubDAGRun, nil
}

// loadDAGWithParams loads the DAG and its parameters from command arguments.
// When isSubDAGRun is true, handlers from base config are skipped to prevent inheritance.
func loadDAGWithParams(ctx *Context, args []string, isSubDAGRun bool) (*core.DAG, string, error) {
	var dagPath string
	var interactiveParams string

	// Check if DAG path is provided
	if len(args) == 0 {
		// Check if we're in an interactive terminal
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			return nil, "", fmt.Errorf("DAG file path is required")
		}

		// Use interactive picker
		logger.Info(ctx, "No DAG specified, opening interactive selector")

		// Get DAG store
		dagStore, err := ctx.dagStore(nil, nil)
		if err != nil {
			return nil, "", fmt.Errorf("failed to initialize DAG store: %w", err)
		}

		// Load DAG metadata first to pass to the picker
		// This will be updated when user selects a DAG
		var tempDAG *core.DAG

		// Show unified interactive UI
		result, err := dagpicker.PickDAGInteractive(ctx, dagStore, tempDAG)
		if err != nil {
			return nil, "", err
		}

		if result.Cancelled {
			fmt.Println("DAG execution aborted.")
			os.Exit(0)
		}

		dagPath = result.DAGPath
		interactiveParams = result.Params
	} else {
		dagPath = args[0]
	}

	// Prepare load options with base configuration
	loadOpts := []spec.LoadOption{
		spec.WithBaseConfig(ctx.Config.Paths.BaseConfig),
		spec.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	}

	// Skip base handlers for sub-DAG runs to prevent handler inheritance.
	// Sub-DAGs should define their own handlers explicitly if needed.
	if isSubDAGRun {
		loadOpts = append(loadOpts, spec.WithSkipBaseHandlers())
	}

	// Get name override from flags if provided
	nameOverride, err := ctx.StringParam("name")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get name override: %w", err)
	}
	if nameOverride != "" {
		loadOpts = append(loadOpts, spec.WithName(nameOverride))
	}

	// Get default working directory from flags if provided
	// This is used for sub-DAG execution to inherit the parent's working directory
	defaultWorkingDir, err := ctx.StringParam("default-working-dir")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get default-working-dir: %w", err)
	}
	if defaultWorkingDir != "" {
		loadOpts = append(loadOpts, spec.WithDefaultWorkingDir(defaultWorkingDir))
	}

	// Load parameters from command line arguments
	var params string

	// Check if parameters are provided after "--"
	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 && len(args) > 0 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, spec.WithParams(args[argsLenAtDash:]))
	} else if interactiveParams != "" {
		// Use interactive parameters
		loadOpts = append(loadOpts, spec.WithParams(stringutil.RemoveQuotes(interactiveParams)))
		params = interactiveParams
	} else {
		// Get parameters from flags
		params, err = ctx.Command.Flags().GetString("params")
		if err != nil {
			return nil, "", fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, spec.WithParams(stringutil.RemoveQuotes(params)))
	}

	// Load the DAG from the specified file
	dag, err := spec.Load(ctx, dagPath, loadOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load DAG from %s: %w", dagPath, err)
	}

	return dag, params, nil
}

// determineRootDAGRun creates or parses the root execution reference
func determineRootDAGRun(isSubDAGRun bool, rootDAGRun string, dag *core.DAG, dagRunID string) (execution.DAGRunRef, error) {
	if isSubDAGRun {
		// Parse the rootDAGRun execution reference for sub dag-runs
		rootDAGRun, err := execution.ParseDAGRunRef(rootDAGRun)
		if err != nil {
			return execution.DAGRunRef{}, fmt.Errorf("failed to parse root exec ref: %w", err)
		}
		return rootDAGRun, nil
	}

	// Create a new root execution reference for root execution
	return execution.NewDAGRunRef(dag.Name, dagRunID), nil
}

// handleSubDAGRun processes a sub dag-run, checking for previous runs
func handleSubDAGRun(ctx *Context, dag *core.DAG, dagRunID string, params string, root execution.DAGRunRef, parent execution.DAGRunRef, workerID string) error {
	// Log sub dag-run execution
	logger.Info(ctx, "Executing sub dag-run",
		slog.String("params", params),
		slog.Any("root", root),
		slog.Any("parent", parent),
	)

	// Double-check dag-run ID is provided (should be caught earlier, but being defensive)
	if dagRunID == "" {
		return fmt.Errorf("dag-run ID must be provided for sub DAGrun")
	}

	// Check for previous sub dag-run with this ID
	logger.Debug(ctx, "Checking for previous sub dag-run with the dag-run ID")

	// Look for existing execution subAttempt
	subAttempt, err := ctx.DAGRunStore.FindSubAttempt(ctx, root, dagRunID)
	if errors.Is(err, execution.ErrDAGRunIDNotFound) {
		// If the dag-run ID is not found, proceed with new execution
		return executeDAGRun(ctx, dag, parent, dagRunID, root, workerID)
	}
	if err != nil {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRunID, err)
	}

	// Read the status of the previous run
	status, err := subAttempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read previous run status for dag-run ID %s: %w", dagRunID, err)
	}

	// Execute as a retry of the previous run
	return executeRetry(ctx, dag, status, root, "", workerID)
}

// executeDAGRun initializes execution state for a DAG run, constructs an agent configured
// with the provided run and topology references, and invokes the shared agent executor.
//
// The function opens (and persists) a log file for the DAG run, ensures the DAG's
// directory is included in the DAG store search path, and creates an agent configured
// with the given parent and root references and the configured peer settings. It then
// calls ExecuteAgent to perform the actual run.
//
// It returns an error if log file initialization, DAG store setup, agent creation, or
// execution fails.
func executeDAGRun(ctx *Context, d *core.DAG, parent execution.DAGRunRef, dagRunID string, root execution.DAGRunRef, workerID string) error {
	// Open the log file for the scheduler. The log file will be used for future
	// execution for the same DAG/dag-run ID between attempts.
	logFile, err := ctx.OpenLogFile(d, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", d.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Debug(ctx, "Dag-run initiated", tag.File(logFile.Name()))

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
		ctx.Config.Core.Peer,
		agent.Options{
			ParentDAGRun:    parent,
			ProgressDisplay: shouldEnableProgress(ctx),
			WorkerID:        workerID,
		},
	)

	// Use the shared agent execution function
	return ExecuteAgent(ctx, agentInstance, d, dagRunID, logFile)
}
