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
	// ErrWorkflowIDRequired is returned when a child workflow is attempted without providing an workflow ID
	ErrWorkflowIDRequired = errors.New("workflow ID is required for child workflow")

	// ErrWorkflowIDFormat is returned when the provided workflow ID has an invalid format
	ErrWorkflowIDFormat = errors.New("workflow ID must only contain alphanumeric characters, dashes, and underscores")

	// ErrWorkflowIDTooLong is returned when the provided workflow ID is too long
	ErrWorkflowIDTooLong = errors.New("workflow ID length must be less than 60 characters")
)

// CmdStart creates and returns a cobra command for starting workflow
func CmdStart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "start [flags] <DAG name> [-- param1 param2 ...]",
			Short: "Execute a workflow",
			Long: `Begin execution of a workflow.

Parameters after the "--" separator are passed as execution parameters (either positional or key=value pairs).
Flags can override default settings such as workflow ID or suppress output.

Example:
  dagu start my_dag -- P1=foo P2=bar

This command parses the DAG definition, resolves parameters, and initiates the execution process.
`,
			Args: cobra.MinimumNArgs(1),
		}, startFlags, runStart,
	)
}

// Command line flags for the start command
var startFlags = []commandLineFlag{paramsFlag, workflowIDFlagStart, parentWorkflowFlag, rootWorkflowFlag}

// runStart handles the execution of the start command
func runStart(ctx *Context, args []string) error {
	// Get workflow ID and references
	workflowID, rootRef, parentRef, isChildWorkflow, err := getExecutionInfo(ctx)
	if err != nil {
		return err
	}

	// Load parameters and DAG
	dag, params, err := loadDAGWithParams(ctx, args)
	if err != nil {
		return err
	}

	// Create or get root execution reference
	root, err := determineRootWorkflow(isChildWorkflow, rootRef, dag, workflowID)
	if err != nil {
		return err
	}

	// Handle child workflow if applicable
	if isChildWorkflow {
		// Parse parent execution reference
		parent, err := digraph.ParseWorkflowRef(parentRef)
		if err != nil {
			return fmt.Errorf("failed to parse parent exec ref: %w", err)
		}
		return handleChildWorkflow(ctx, dag, workflowID, params, root, parent)
	}

	// Log root workflow
	logger.Info(ctx, "Executing root DAG",
		"name", dag.Name,
		"params", params,
		"workflowId", workflowID,
	)

	// Execute the DAG
	return executeWorkflow(ctx, dag, digraph.WorkflowRef{}, workflowID, root)
}

// getExecutionInfo extracts and validates workflow ID and references from command flags
func getExecutionInfo(ctx *Context) (workflowID string, rootRef string, parentRef string, isChildWorkflow bool, err error) {
	// Get workflow ID from flags
	workflowID, err = ctx.Command.Flags().GetString("workflow-id")
	if err != nil {
		return "", "", "", false, fmt.Errorf("failed to get workflow ID: %w", err)
	}

	// Get root and parent execution references
	rootRef, _ = ctx.Command.Flags().GetString("root")
	parentRef, _ = ctx.Command.Flags().GetString("parent")
	isChildWorkflow = parentRef != "" || rootRef != ""

	// Validate workflow ID for child workflows
	if isChildWorkflow && workflowID == "" {
		return "", "", "", false, ErrWorkflowIDRequired
	}

	// Validate or generate workflow ID
	if workflowID != "" {
		if err := validateWorkflowID(workflowID); err != nil {
			return "", "", "", false, err
		}
	} else {
		// Generate a new workflow ID if not provided
		workflowID, err = getWorkflowID()
		if err != nil {
			return "", "", "", false, fmt.Errorf("failed to generate workflow ID: %w", err)
		}
	}

	return workflowID, rootRef, parentRef, isChildWorkflow, nil
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

// determineRootWorkflow creates or parses the root execution reference
func determineRootWorkflow(isChildWorkflow bool, rootRef string, dag *digraph.DAG, workflowID string) (digraph.WorkflowRef, error) {
	if isChildWorkflow {
		// Parse the root execution reference for child workflow
		root, err := digraph.ParseWorkflowRef(rootRef)
		if err != nil {
			return digraph.WorkflowRef{}, fmt.Errorf("failed to parse root exec ref: %w", err)
		}
		return root, nil
	}

	// Create a new root execution reference for root execution
	return digraph.NewWorkflowRef(dag.Name, workflowID), nil
}

// handleChildWorkflow processes a child workflow, checking for previous runs
func handleChildWorkflow(ctx *Context, dag *digraph.DAG, workflowID string, params string, root digraph.WorkflowRef, parent digraph.WorkflowRef) error {
	// Log child workflow
	logger.Info(ctx, "Executing child workflow",
		"name", dag.Name,
		"params", params,
		"workflowId", workflowID,
		"root", root,
		"parent", parent,
	)

	// Double-check workflow ID is provided (should be caught earlier, but being defensive)
	if workflowID == "" {
		return fmt.Errorf("workflow ID must be provided for child workflow")
	}

	// Check for previous child workflow with this ID
	logger.Debug(ctx, "Checking for previous child workflow with the workflow ID", "workflowId", workflowID)

	// Look for existing execution run
	run, err := ctx.HistoryRepo.FindChildWorkflowRun(ctx, root, workflowID)
	if errors.Is(err, models.ErrWorkflowIDNotFound) {
		// If the workflow ID is not found, proceed with new execution
		return executeWorkflow(ctx, dag, parent, workflowID, root)
	}
	if err != nil {
		return fmt.Errorf("failed to find the record for workflow ID %s: %w", workflowID, err)
	}

	// Read the status of the previous run
	status, err := run.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read previous run status for workflow ID %s: %w", workflowID, err)
	}

	// Execute as a retry of the previous run
	return executeRetry(ctx, dag, status, root)
}

// executeWorkflow handles the actual execution of a DAG
func executeWorkflow(ctx *Context, d *digraph.DAG, parent digraph.WorkflowRef, workflowID string, root digraph.WorkflowRef) error {
	// Open the log file for the scheduler. The log file will be used for future
	// execution for the same DAG/workflow ID between attempts.
	logFile, err := ctx.OpenLogFile(d, workflowID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", d.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	// Configure logging to the file
	ctx.LogToFile(logFile)

	logger.Debug(ctx, "workflow initiated", "DAG", d.Name, "workflowId", workflowID, "logFile", logFile.Name())

	// Initialize DAG repository with the DAG's directory in the search path
	dr, err := ctx.dagRepo(nil, []string{filepath.Dir(d.Location)})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	// Create a new agent to execute the DAG
	agentInstance := agent.New(
		workflowID,
		d,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.HistoryMgr,
		dr,
		ctx.HistoryRepo,
		root,
		agent.Options{Parent: parent},
	)

	// Set up signal handling for the agent
	listenSignals(ctx, agentInstance)

	// Run the DAG
	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute the workflow", "name", d.Name, "workflowId", workflowID, "err", err)

		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute the workflow %s (workflow ID: %s): %w", d.Name, workflowID, err)
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
