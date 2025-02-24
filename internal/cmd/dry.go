package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/spf13/cobra"
)

func CmdDry() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] /path/to/spec.yaml",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry /path/to/spec.yaml -- params1 params2`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  wrapRunE(runDry),
	}
	initFlags(cmd, paramsFlag)
	return cmd
}

func runDry(cmd *cobra.Command, args []string) error {
	setup, err := createSetup(cmd.Context(), false)
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

	ctx := setup.ctx
	dag, err := digraph.Load(ctx, args[0], loadOpts...)
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	const logPrefix = "dry_"
	logFile, err := setup.OpenLogFile(ctx, logPrefix, dag, requestID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	ctx = setup.loggerContextWithFile(ctx, false, logFile)

	dagStore, err := setup.dagStore()
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	cli, err := setup.Client()
	if err != nil {
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
		agent.Options{Dry: true},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
	}

	agentInstance.PrintSummary(ctx)

	return nil
}
