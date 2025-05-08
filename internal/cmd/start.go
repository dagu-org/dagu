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

var startFlags = []commandLineFlag{paramsFlag, reqIDFlagStart, parentReqIDFlag, rootDAGNameFlag, rootReqIDFlag}

func runStart(ctx *Context, args []string) error {
	reqID, err := ctx.Command.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	// Generate requestID if it's not specified.
	if reqID == "" {
		var err error
		reqID, err = genReqID()
		if err != nil {
			logger.Error(ctx, "Failed to generate request ID", "err", err)
			return fmt.Errorf("failed to generate request ID: %w", err)
		}
	} else if err := validateReqID(reqID); err != nil {
		logger.Error(ctx, "Invalid request ID format", "reqId", reqID, "err", err)
		return fmt.Errorf("invalid request ID format: %w", err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(ctx.Config.Paths.BaseConfig),
		digraph.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	}

	// Load parameters from command line arguments.
	var params string
	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, digraph.WithParams(args[argsLenAtDash:]))
	} else {
		// Get parameters from flags
		params, err = ctx.Command.Flags().GetString("params")
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

	rootRequestID, _ := ctx.Command.Flags().GetString("root-request-id")
	rootDAGName, _ := ctx.Command.Flags().GetString("root-dag-name")
	parentRequestID, _ := ctx.Command.Flags().GetString("parent-request-id")

	// If rootDAGName is not empty, it means current execution is a sub-DAG.
	// Sub DAG execution requires both root-request-id and root-dag-name to be set.
	if (rootRequestID == "" && rootDAGName != "") || (rootRequestID != "" && rootDAGName == "") {
		return fmt.Errorf("both root-request-id and root-dag-name must be provided together or neither should be provided")
	}

	var rootRun digraph.RootRun
	if rootDAGName != "" && rootRequestID != "" {
		// The current execution is a sub-DAG
		rootRun = digraph.NewRootRun(rootDAGName, rootRequestID)
		logger.Info(ctx, "Executing sub-DAG",
			"dagName", dag.Name,
			"params", params,
			"reqId", reqID,
			"rootDAGName", rootDAGName,
			"rootReqID", rootRequestID,
			"parentRequestID", parentRequestID,
		)
	} else {
		// The current execution is a root DAG
		rootRun = digraph.NewRootRun(dag.Name, reqID)
		logger.Info(ctx, "Executing root DAG",
			"dagName", dag.Name,
			"params", params,
			"reqId", reqID,
		)
	}

	// Check for previous runs with this request ID and retry it if found.
	// This prevents duplicate execution when retrying or when sub-DAGs share the
	// same request ID, ensuring idempotency across the the DAG from the root DAG.
	if rootRun.ReqID != reqID {
		if reqID == "" {
			logger.Error(ctx, "Request ID must be provided for sub-DAG run")
			return fmt.Errorf("request ID must be provided for sub-DAG run")
		}
		logger.Debug(ctx, "Checking for previous sub-DAG run with the request ID", "reqId", reqID)
		var status *models.Status
		record, err := ctx.HistoryRepo.FindSubRun(ctx, rootRun.Name, rootRun.ReqID, reqID)
		if errors.Is(err, models.ErrReqIDNotFound) {
			// If the request ID is not found, proceed with execution
			goto EXEC
		}
		if err != nil {
			logger.Error(ctx, "Failed to retrieve historical run", "reqId", reqID, "err", err)
			return fmt.Errorf("failed to retrieve historical run for request ID %s: %w", reqID, err)
		}
		status, err = record.ReadStatus(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to read previous run status", "reqId", reqID, "err", err)
			return fmt.Errorf("failed to read previous run status for request ID %s: %w", reqID, err)
		}
		return executeRetry(ctx, dag, status, rootRun)
	}

EXEC:
	return executeDag(ctx, dag, parentRequestID, reqID, rootRun)
}

func executeDag(ctx *Context, d *digraph.DAG, parentReqID, reqID string, rootRun digraph.RootRun) error {
	// Open the log file for the scheduler. The log file will be used for future
	// execution for the same DAG/request ID between attempts.
	logFile, err := ctx.OpenLogFile(d, reqID)
	if err != nil {
		logger.Error(ctx, "failed to initialize log file", "DAG", d.Name, "err", err)
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", d.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	logger.Debug(ctx, "DAG run initiated", "DAG", d.Name, "reqId", reqID, "logFile", logFile.Name())

	dr, err := ctx.dagRepo(nil, []string{filepath.Dir(d.Location)})
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		reqID,
		d,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.HistoryMgr,
		dr,
		ctx.HistoryRepo,
		rootRun,
		agent.Options{ParentID: parentReqID},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute DAG", "DAG", d.Name, "reqId", reqID, "err", err)

		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", d.Name, reqID, err)
		}
	}

	// Print the summary of the execution if the quiet flag is not set.
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
