package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdStart() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] /path/to/spec.yaml [-- params1 params2]",
		Short: "Runs the DAG",
		Long:  `dagu start /path/to/spec.yaml -- params1 params2`,
		Args:  cobra.MinimumNArgs(1),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return bindCommonFlags(cmd, nil)
		},
		RunE: wrapRunE(runStart),
	}

	initStartFlags(cmd)
	return cmd
}

func initStartFlags(cmd *cobra.Command) {
	initCommonFlags(cmd, []commandLineFlag{paramsFlag, withUsage(requestIDFlag, "request ID for the DAG execution")})
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
}

func runStart(cmd *cobra.Command, args []string) error {
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return fmt.Errorf("failed to get quiet flag: %w", err)
	}

	requestID, err := cmd.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	setup, err := createSetup(cmd.Context(), quiet)
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(setup.cfg.Paths.BaseConfig),
	}

	var params string
	if argsLenAtDash := cmd.ArgsLenAtDash(); argsLenAtDash != -1 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, digraph.WithParams(args[argsLenAtDash:]))
	} else {
		// Get parameters from flags
		params, err = cmd.Flags().GetString("params")
		if err != nil {
			return fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, digraph.WithParams(removeQuotes(params)))
	}

	return executeDag(setup.ctx, setup, args[0], loadOpts, quiet, requestID)
}

func executeDag(ctx context.Context, setup *Setup, specPath string, loadOpts []digraph.LoadOption, quiet bool, requestID string) error {
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
	logFile, err := setup.OpenLogFile(ctx, logPrefix, dag, requestID)
	if err != nil {
		logger.Error(ctx, "failed to initialize log file", "DAG", dag.Name, "err", err)
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	ctx = setup.loggerContextWithFile(ctx, quiet, logFile)

	logger.Info(ctx, "DAG execution initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())

	dagStore, err := setup.dagStore()
	if err != nil {
		logger.Error(ctx, "Failed to initialize DAG store", "err", err)
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	cli, err := setup.Client()
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
		setup.historyStore(),
		agent.Options{},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute DAG", "DAG", dag.Name, "requestID", requestID, "err", err)

		if quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
		}
	}

	if !quiet {
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
