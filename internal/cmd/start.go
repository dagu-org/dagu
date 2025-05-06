package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/runstore"
	"github.com/spf13/cobra"
)

func CmdStart() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "start [flags] /path/to/spec.yaml [-- param1 param2 ...]",
			Short: "Execute a DAG",
			Long: `Begin execution of a DAG defined in a YAML file.

Parameters after the "--" separator are passed as execution parameters (either positional or key=value pairs).
Flags can override default settings such as request ID or suppress output.

Example:
  dagu start my_dag.yaml -- P1=foo P2=bar

This command parses the DAG specification, resolves parameters, and initiates the execution process.
`,
			Args: cobra.MinimumNArgs(1),
		}, startFlags, runStart,
	)
}

var startFlags = []commandLineFlag{paramsFlag, requestIDFlagStart, rootDAGNameFlag, rootRequestIDFlag}

func runStart(ctx *Context, args []string) error {
	requestID, err := ctx.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	// Generate requestID if it's not specified.
	if requestID == "" {
		var err error
		requestID, err = generateRequestID()
		if err != nil {
			logger.Error(ctx, "Failed to generate request ID", "err", err)
			return fmt.Errorf("failed to generate request ID: %w", err)
		}
	} else if err := validateRequestID(requestID); err != nil {
		logger.Error(ctx, "Invalid request ID format", "requestID", requestID, "err", err)
		return fmt.Errorf("invalid request ID format: %w", err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(ctx.cfg.Paths.BaseConfig),
		digraph.WithDAGsDir(ctx.cfg.Paths.DAGsDir),
	}

	// Load parameters from command line arguments.
	var params string
	if argsLenAtDash := ctx.ArgsLenAtDash(); argsLenAtDash != -1 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, digraph.WithParams(args[argsLenAtDash:]))
	} else {
		// Get parameters from flags
		params, err = ctx.Flags().GetString("params")
		if err != nil {
			return fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, digraph.WithParams(removeQuotes(params)))
	}

	// Load the DAG from the specified file
	dag, err := digraph.Load(ctx, args[0], loadOpts...)
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "path", args[0], "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	rootRequestID, _ := ctx.Flags().GetString("root-request-id")
	rootDAGName, _ := ctx.Flags().GetString("root-dag-name")

	// If rootDAGName is not empty, it means current execution is a sub-DAG.
	// Sub DAG execution requires both root-request-id and root-dag-name to be set.
	if (rootRequestID == "" && rootDAGName != "") || (rootRequestID != "" && rootDAGName == "") {
		return fmt.Errorf("both root-request-id and root-dag-name must be provided together or neither should be provided")
	}

	var rootDAG digraph.RootDAG
	if rootDAGName != "" && rootRequestID != "" {
		// The current execution is a sub-DAG
		logger.Debug(ctx, "Sub-DAG execution detected", "rootDAGName", rootDAGName, "rootRequestID", rootRequestID)
		rootDAG = digraph.NewRootDAG(rootDAGName, rootRequestID)
	} else {
		// The current execution is a root DAG
		rootDAG = digraph.NewRootDAG(dag.Name, requestID)
	}

	// Check for previous runs with this request ID and retry it if found.
	// This prevents duplicate execution when retrying or when sub-DAGs share the
	// same request ID, ensuring idempotency across the the DAG from the root DAG.
	if rootDAG.RequestID != requestID {
		logger.Debug(ctx, "Checking for previous sub-DAG run with the request ID", "requestID", requestID)
		var status *runstore.Status
		record, err := ctx.runStore().FindBySubRunRequestID(ctx, requestID, rootDAG)
		if errors.Is(err, runstore.ErrRequestIDNotFound) {
			// If the request ID is not found, proceed with execution
			goto EXEC
		}
		if err != nil {
			logger.Error(ctx, "Failed to retrieve historical run", "requestID", requestID, "err", err)
			return fmt.Errorf("failed to retrieve historical run for request ID %s: %w", requestID, err)
		}
		status, err = record.ReadStatus(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to read previous run status", "requestID", requestID, "err", err)
			return fmt.Errorf("failed to read previous run status for request ID %s: %w", requestID, err)
		}
		return executeRetry(ctx, dag, status, rootDAG)
	}

EXEC:
	return executeDag(ctx, dag, requestID, rootDAG)
}

func executeDag(ctx *Context, dag *digraph.DAG, requestID string, rootDAG digraph.RootDAG) error {
	logger.Debug(ctx, "Executing DAG", "dagName", dag.Name, "requestID", requestID)

	// Open the log file for the scheduler. The log file will be used for future
	// execution for the same DAG/request ID between attempts.
	logFile, err := ctx.OpenLogFile(dag, requestID)
	if err != nil {
		logger.Error(ctx, "failed to initialize log file", "DAG", dag.Name, "err", err)
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	logger.Debug(ctx, "DAG run initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())

	dagStore, err := ctx.dagStore([]string{filepath.Dir(dag.Location)})
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	cli, err := ctx.Client()
	if err != nil {
		logger.Error(ctx, "Failed to initialize client", "err", err)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	var opts agent.Options
	agentInstance := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		ctx.runStore(),
		rootDAG,
		opts,
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute DAG", "DAG", dag.Name, "requestID", requestID, "err", err)

		if ctx.quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
		}
	}

	// Print the summary of the execution if the quiet flag is not set.
	if !ctx.quiet {
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
