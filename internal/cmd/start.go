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

var startFlags = []commandLineFlag{paramsFlag, requestIDFlagStart}

func runStart(cmd *Command, args []string) error {
	requestID, err := cmd.cmd.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(cmd.cfg.Paths.BaseConfig),
	}

	var params string
	if argsLenAtDash := cmd.cmd.ArgsLenAtDash(); argsLenAtDash != -1 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, digraph.WithParams(args[argsLenAtDash:]))
	} else {
		// Get parameters from flags
		params, err = cmd.cmd.Flags().GetString("params")
		if err != nil {
			return fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, digraph.WithParams(removeQuotes(params)))
	}

	return executeDag(cmd, args[0], loadOpts, requestID)
}

func executeDag(cmd *Command, specPath string, loadOpts []digraph.LoadOption, requestID string) error {
	dag, err := digraph.Load(cmd.ctx, specPath, loadOpts...)
	if err != nil {
		logger.Error(cmd.ctx, "Failed to load DAG", "path", specPath, "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", specPath, err)
	}

	if requestID == "" {
		var err error
		requestID, err = generateRequestID()
		if err != nil {
			logger.Error(cmd.ctx, "Failed to generate request ID", "err", err)
			return fmt.Errorf("failed to generate request ID: %w", err)
		}
	}

	const logPrefix = "start_"
	logFile, err := cmd.OpenLogFile(cmd.ctx, logPrefix, dag, requestID)
	if err != nil {
		logger.Error(cmd.ctx, "failed to initialize log file", "DAG", dag.Name, "err", err)
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	ctx := cmd.loggerContextWithFile(logFile)

	logger.Info(ctx, "DAG execution initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())

	dagStore, err := cmd.dagStore()
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	cli, err := cmd.Client()
	if err != nil {
		logger.Error(ctx, "Failed to initialize client", "err", err)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	agentInstance := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		cmd.historyStore(),
		agent.Options{},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute DAG", "DAG", dag.Name, "requestID", requestID, "err", err)

		if cmd.quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
		}
	}

	if !cmd.quiet {
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
