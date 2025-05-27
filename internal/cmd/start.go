package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"

	"github.com/dagu-org/dagu/internal/stringutil"
)

// Errors for start command
var (
	// ErrDAGRunIDRequired is returned when a child dag-run is attempted without providing a dag-run ID
	ErrDAGRunIDRequired = errors.New("dag-run ID must be provided for child dag-runs")

	// ErrDAGRunIDFormat is returned when the provided dag-run ID is not valid
	ErrDAGRunIDFormat = errors.New("dag-run ID must only contain alphanumeric characters, dashes, and underscores")

	// ErrDAGRunIDTooLong is returned when the provided dag-run ID is too long
	ErrDAGRunIDTooLong = errors.New("dag-run ID length must be less than 60 characters")
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
			Args: cobra.MinimumNArgs(1),
		}, startFlags, runStart,
	)
}

// Command line flags for the start command
var startFlags = []commandLineFlag{paramsFlag, dagRunIDFlag, parentDAGRunFlag, rootDAGRunFlag}

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

	// Log root dag-run
	logger.Info(ctx, "Executing root dag-run",
		"dag", dag.Name,
		"params", params,
		"dagRunId", dagRunID,
	)

	// Execute the dag-run
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
	if len(args) == 0 {
		return nil, "", fmt.Errorf("DAG file path is required")
	}

	// Prepare load options with base configuration
	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(ctx.Config.Paths.BaseConfig),
		digraph.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	}

	// Load parameters from command line arguments
	var params string
	var err error

	// Check if parameters are provided after "--"
	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, digraph.WithParams(args[argsLenAtDash:]))
	} else {
		// Get parameters from flags
		params, err = ctx.Command.Flags().GetString("params")
		if err != nil {
			return nil, "", fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, digraph.WithParams(stringutil.RemoveQuotes(params)))
	}

	// Load the DAG from the specified file
	dag, err := digraph.Load(ctx, args[0], loadOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
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
	return executeRetry(ctx, dag, status, root)
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

	// Configure logging to the file
	ctx.LogToFile(logFile)

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
		ctx.ProcStore,
		root,
		agent.Options{ParentDAGRun: parent},
	)

	// Set up signal handling for the agent
	listenSignals(ctx, agentInstance)

	// Run the DAG
	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute dag-run", "dag", d.Name, "dagRunId", dagRunID, "err", err)

		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute the dag-run %s (dag-run ID: %s): %w", d.Name, dagRunID, err)
		}
	}

	// Print the summary of the execution if the quiet flag is not set
	if !ctx.Quiet {
		agentInstance.PrintSummary(ctx)
	}

	return nil
}
