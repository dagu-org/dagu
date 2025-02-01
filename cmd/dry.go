package main

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/spf13/cobra"
)

const (
	dryPrefix = "dry_"
)

func dryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dry [flags] /path/to/spec.yaml",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry /path/to/spec.yaml -- params1 params2`,
		Args:  cobra.MinimumNArgs(1),
		RunE:  wrapRunE(runDry),
	}
}

func runDry(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	setup := newSetup(cfg)

	cmd.Flags().StringP("params", "p", "", "parameters")

	ctx := setup.loggerContext(cmd.Context(), false)

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

	dag, err := digraph.Load(ctx, args[0], loadOpts...)
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	logFile, err := setup.openLogFile(ctx, dryPrefix, dag, requestID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	ctx = setup.loggerContextWithFile(ctx, false, logFile)

	dagStore, err := setup.dagStore()
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	cli, err := setup.client()
	if err != nil {
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	agt := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dagStore,
		setup.historyStore(),
		agent.Options{Dry: true},
	)

	listenSignals(ctx, agt)

	if err := agt.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
	}

	agt.PrintSummary(ctx)

	return nil
}
