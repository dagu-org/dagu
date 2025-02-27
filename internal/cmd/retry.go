package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/spf13/cobra"
)

func CmdRetry() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "retry [flags] /path/to/spec.yaml",
			Short: "Retry a DAG execution",
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

	specFilePath := args[0]

	absolutePath, err := filepath.Abs(specFilePath)
	if err != nil {
		logger.Error(ctx, "Failed to resolve absolute path", "path", specFilePath, "err", err)
		return fmt.Errorf("failed to resolve absolute path for %s: %w", specFilePath, err)
	}

	status, err := ctx.historyStore().FindByRequestID(ctx, absolutePath, requestID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve historical execution", "requestID", requestID, "err", err)
		return fmt.Errorf("failed to retrieve historical execution for request ID %s: %w", requestID, err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(ctx.cfg.Paths.BaseConfig),
	}

	if status.Status.Params != "" {
		// backward compatibility
		loadOpts = append(loadOpts, digraph.WithParams(status.Status.Params))
	} else {
		loadOpts = append(loadOpts, digraph.WithParams(status.Status.ParamsList))
	}

	dag, err := digraph.Load(ctx, absolutePath, loadOpts...)
	if err != nil {
		logger.Error(ctx, "Failed to load DAG specification", "path", specFilePath, "err", err)
		// nolint : staticcheck
		return fmt.Errorf("failed to load DAG specification from %s with params %s: %w",
			specFilePath, status.Status.Params, err)
	}

	// Execute DAG retry
	if err := executeRetry(ctx, dag, status); err != nil {
		logger.Error(ctx, "Failed to execute retry", "path", specFilePath, "err", err)
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

func executeRetry(ctx *Context, dag *digraph.DAG, originalStatus *persistence.StatusFile) error {
	newRequestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate new request ID: %w", err)
	}

	const logPrefix = "retry_"
	logFile, err := ctx.OpenLogFile(logPrefix, dag, newRequestID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	logger.Info(ctx, "DAG retry initiated", "DAG", dag.Name, "originalRequestID", originalStatus.Status.RequestID, "newRequestID", newRequestID, "logFile", logFile.Name())

	ctx.LogToFile(logFile)

	dagStore, err := ctx.dagStore()
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
		newRequestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		ctx.historyStore(),
		agent.Options{RetryTarget: &originalStatus.Status},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		if ctx.quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, newRequestID, err)
		}
	}

	if !ctx.quiet {
		agentInstance.PrintSummary(ctx)
	}

	return nil
}
