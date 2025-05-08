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
)

// Errors for start command
var (
	// ErrExecIDRequired is returned when a child execution is attempted without providing an execution ID
	ErrExecIDRequired = errors.New("execution ID is required for child execution")

	// ErrExecIDFormat is returned when the provided execution ID has an invalid format
	ErrExecIDFormat = errors.New("invalid execution ID format")
)

// CmdStart creates and returns a cobra command for starting DAG execution
func CmdStart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "start [flags] /path/to/spec.yaml [-- param1 param2 ...]",
			Short: "Execute a DAG",
			Long: `Begin execution of a DAG defined in a YAML file.

Parameters after the "--" separator are passed as execution parameters (either positional or key=value pairs).
Flags can override default settings such as execution ID or suppress output.

Example:
  dagu start my_dag.yaml -- P1=foo P2=bar

This command parses the DAG specification, resolves parameters, and initiates the execution process.
`,
			Args: cobra.MinimumNArgs(1),
		}, startFlags, runStart,
	)
}

// Command line flags for the start command
var startFlags = []commandLineFlag{paramsFlag, execIDFlagStart, parentFlag, rootFlag}

// runStart handles the execution of the start command
func runStart(ctx *Context, args []string) error {
	// Get execution ID and references
	execID, rootRef, parentRef, isChildExec, err := getExecutionInfo(ctx)
	if err != nil {
		return err
	}

	// Load parameters and DAG
	dag, params, err := loadDAGWithParams(ctx, args)
	if err != nil {
		return err
	}

	// Create or get root execution reference
	root, err := determineRootExecRef(isChildExec, rootRef, dag, execID)
	if err != nil {
		return err
	}

	// Handle child execution if applicable
	if isChildExec {
		return handleChildExecution(ctx, dag, execID, params, root, parentRef)
	}

	// Log root DAG execution
	logger.Info(ctx, "Executing root DAG",
		"name", dag.Name,
		"params", params,
		"execId", execID,
	)

	// Execute the DAG
	return executeDag(ctx, dag, digraph.ExecRef{}, execID, root)
}

// getExecutionInfo extracts and validates execution ID and references from command flags
func getExecutionInfo(ctx *Context) (execID string, rootRef string, parentRef string, isChildExec bool, err error) {
	// Get execution ID from flags
	execID, err = ctx.Command.Flags().GetString("exec-id")
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get execution ID: %w", err)
	}

	// Get root and parent execution references
	rootRef, _ = ctx.Command.Flags().GetString("root")
	parentRef, _ = ctx.Command.Flags().GetString("parent")
	isChildExec = parentRef != "" || rootRef != ""

	// Validate execution ID for child executions
	if isChildExec && execID == "" {
		return "", "", "", false, ErrExecIDRequired
	}

	// Validate or generate execution ID
	if execID != "" {
		if err := validateExecID(execID); err != nil {
			return "", "", "", false, ErrExecIDFormat
		}
	} else {
		// Generate a new execution ID if not provided
		execID, err = genReqID()
		if err != nil {
			return "", "", "", false, fmt.Errorf("failed to generate execution ID: %w", err)
		}
	}

	return execID, rootRef, parentRef, isChildExec, nil
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
		loadOpts = append(loadOpts, digraph.WithParams(removeQuotes(params)))
	}

	// Load the DAG from the specified file
	dag, err := digraph.Load(ctx, args[0], loadOpts...)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	return dag, params, nil
}

// determineRootExecRef creates or parses the root execution reference
func determineRootExecRef(isChildExec bool, rootExecRefID string, dag *digraph.DAG, execID string) (digraph.ExecRef, error) {
	if isChildExec {
		// Parse the root execution reference for child execution
		root, err := digraph.ParseExecRef(rootExecRefID)
		if err != nil {
			return digraph.ExecRef{}, fmt.Errorf("failed to parse root exec ref: %w", err)
		}
		return root, nil
	}

	// Create a new root execution reference for root execution
	return digraph.NewExecRef(dag.Name, execID), nil
}

// handleChildExecution processes a child DAG execution, checking for previous runs
func handleChildExecution(ctx *Context, dag *digraph.DAG, execID string, params string, root digraph.ExecRef, parentRefID string) error {
	// Parse parent execution reference
	parent, err := digraph.ParseExecRef(parentRefID)
	if err != nil {
		return fmt.Errorf("failed to parse parent exec ref: %w", err)
	}

	// Log child DAG execution
	logger.Info(ctx, "Executing child DAG",
		"name", dag.Name,
		"params", params,
		"execId", execID,
		"root", root,
		"parent", parent,
	)

	// Double-check execution ID is provided (should be caught earlier, but being defensive)
	if execID == "" {
		return fmt.Errorf("execution ID must be provided for child execution")
	}

	// Check for previous child execution with this ID
	logger.Debug(ctx, "Checking for previous child execution with the execution ID", "execId", execID)

	// Look for existing execution record
	record, err := ctx.HistoryRepo.FindChildExecution(ctx, root, execID)
	if errors.Is(err, models.ErrExecIDNotFound) {
		// If the execution ID is not found, proceed with new execution
		return executeDag(ctx, dag, parent, execID, root)
	}
	if err != nil {
		return fmt.Errorf("failed to find the record for execution ID %s: %w", execID, err)
	}

	// Read the status of the previous run
	status, err := record.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read previous run status for execution ID %s: %w", execID, err)
	}

	// Execute as a retry of the previous run
	return executeRetry(ctx, dag, status, root)
}

// executeDag handles the actual execution of a DAG
func executeDag(ctx *Context, d *digraph.DAG, parent digraph.ExecRef, reqID string, rootRun digraph.ExecRef) error {
	// Open the log file for the scheduler. The log file will be used for future
	// execution for the same DAG/execution ID between attempts.
	logFile, err := ctx.OpenLogFile(d, reqID)
	if err != nil {
		logger.Error(ctx, "failed to initialize log file", "DAG", d.Name, "err", err)
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", d.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	// Configure logging to the file
	ctx.LogToFile(logFile)

	logger.Debug(ctx, "DAG run initiated", "DAG", d.Name, "execId", reqID, "logFile", logFile.Name())

	// Initialize DAG repository with the DAG's directory in the search path
	dr, err := ctx.dagRepo(nil, []string{filepath.Dir(d.Location)})
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	// Create a new agent to execute the DAG
	agentInstance := agent.New(
		reqID,
		d,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.HistoryMgr,
		dr,
		ctx.HistoryRepo,
		rootRun,
		agent.Options{Parent: parent},
	)

	// Set up signal handling for the agent
	listenSignals(ctx, agentInstance)

	// Run the DAG
	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute DAG", "DAG", d.Name, "execId", reqID, "err", err)

		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", d.Name, reqID, err)
		}
	}

	// Print the summary of the execution if the quiet flag is not set
	if !ctx.Quiet {
		agentInstance.PrintSummary(ctx)
	}

	return nil
}

// removeQuotes removes the surrounding quotes from the string.
func removeQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
