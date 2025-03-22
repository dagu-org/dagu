package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
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

	rootRequestID, _ := ctx.Flags().GetString("root-request-id")
	rootDAGName, _ := ctx.Flags().GetString("root-dag-name")

	// Validate consistency between rootRequestID and rootDAGName
	if (rootRequestID == "" && rootDAGName != "") || (rootRequestID != "" && rootDAGName == "") {
		return fmt.Errorf("both root-request-id and root-dag-name must be provided together or neither should be provided")
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(ctx.cfg.Paths.BaseConfig),
	}

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

	return executeDag(ctx, args[0], loadOpts, requestID, rootDAGName, rootRequestID)
}

func executeDag(ctx *Context, specPath string, loadOpts []digraph.LoadOption, requestID, rootDAGName, rootRequestID string) error {
	dag, err := digraph.Load(ctx, specPath, loadOpts...)
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "path", specPath, "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", specPath, err)
	}

	if requestID == "" {
		var err error
		requestID, err = generateRequestID()
		if err != nil {
			logger.Error(ctx, "Failed to generate request ID", "err", err)
			return fmt.Errorf("failed to generate request ID: %w", err)
		}
	}

	const logPrefix = "start_"
	logFile, err := ctx.OpenLogFile(logPrefix, dag, requestID)
	if err != nil {
		logger.Error(ctx, "failed to initialize log file", "DAG", dag.Name, "err", err)
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	ctx.LogToFile(logFile)

	logger.Debug(ctx, "DAG execution initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())

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

	var opts agent.Options
	if rootDAGName != "" && rootRequestID != "" {
		opts.RootDAG = &digraph.RootDAG{
			Name:      rootDAGName,
			RequestID: rootRequestID,
		}
	}

	agentInstance := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		ctx.historyStore(),
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
