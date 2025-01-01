// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

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
		Long:  `dagu dry [--params="param1 param2"] /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
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
	params, err := cmd.Flags().GetString("params")
	if err != nil {
		return fmt.Errorf("failed to get parameters: %w", err)
	}

	ctx := cmd.Context()

	dag, err := digraph.Load(ctx, cfg.Paths.BaseConfig, args[0], removeQuotes(params))
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	logFile, err := setup.openLogFile(dryPrefix, dag, requestID)
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
		&agent.Options{Dry: true},
	)

	listenSignals(ctx, agt)

	if err := agt.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
	}

	return nil
}
