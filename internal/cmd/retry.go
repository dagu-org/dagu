package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/runstore"
	"github.com/spf13/cobra"
)

func CmdRetry() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "retry [flags] /path/to/spec.yaml",
			Short: "Retry a DAG run",
			Long: `Re-execute a previously run DAG using its unique request ID.

Example:
  dagu retry my_dag.yaml --request-id=abc123

This command is useful for recovering from errors or transient issues by re-running the DAG.
`,
			Args: cobra.ExactArgs(1),
		}, retryFlags, runRetry,
	)
}

var retryFlags = []commandLineFlag{requestIDFlagRetry}

func runRetry(ctx *Context, args []string) error {
	requestID, err := ctx.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	dagName := args[0]

	// Retrieve the previous run's runstore record for the specified request ID.
	runRecord, err := ctx.runStore().Find(ctx, dagName, requestID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve historical run", "requestID", requestID, "err", err)
		return fmt.Errorf("failed to retrieve historical run for request ID %s: %w", requestID, err)
	}

	// Read the detailed status of the previous status.
	status, err := runRecord.ReadStatus(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read status", "err", err)
		return fmt.Errorf("failed to read status: %w", err)
	}

	// Get the DAG instance from the runstore record.
	dag, err := runRecord.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG from runstore record", "err", err)
	}

	// The retry command is currently only supported for root DAGs.
	// Therefore we use the request ID as the root DAG request ID here.
	rootDAG := digraph.NewRootDAG(dag.Name, status.RequestID)

	if err := executeRetry(ctx, dag, status, rootDAG); err != nil {
		logger.Error(ctx, "Failed to execute retry", "path", dagName, "err", err)
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

func executeRetry(ctx *Context, dag *digraph.DAG, status *runstore.Status, rootDAG digraph.RootDAG) error {
	logger.Debug(ctx, "Executing retry", "dagName", dag.Name, "requestID", status.RequestID)

	// We use the same log file for the retry as the original run.
	logFile, err := OpenOrCreateLogFile(status.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Info(ctx, "DAG retry initiated", "DAG", dag.Name, "requestID", status.RequestID, "logFile", logFile.Name())

	// Update the context with the log file
	ctx.LogToFile(logFile)

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

	agentInstance := agent.New(
		status.RequestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		ctx.runStore(),
		rootDAG,
		agent.Options{
			RetryTarget: status,
			ParentID:    status.ParentID,
		},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		if ctx.quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, status.RequestID, err)
		}
	}

	// Print the summary of the execution if the quiet flag is not set.
	if !ctx.quiet {
		agentInstance.PrintSummary(ctx)
	}

	return nil
}
