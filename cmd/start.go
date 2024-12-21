// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

const startPrefix = "start_"

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] /path/to/spec.yaml",
		Short: "Runs the DAG",
		Long:  `dagu start [--params="param1 param2"] /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		RunE:  runStart,
	}

	initStartFlags(cmd)
	return cmd
}

func initStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("params", "p", "", "parameters")
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
}

func runStart(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Get quiet flag
	quiet, err := cmd.Flags().GetBool("quiet")
	if err != nil {
		return fmt.Errorf("failed to get quiet flag: %w", err)
	}

	ctx := cmd.Context()
	ctx = logger.WithLogger(ctx, buildLogger(cfg))

	// Get parameters
	params, err := cmd.Flags().GetString("params")
	if err != nil {
		return fmt.Errorf("failed to get parameters: %w", err)
	}

	// Initialize and run DAG
	return executeDag(ctx, cfg, args[0], removeQuotes(params), quiet)
}

func executeDag(ctx context.Context, cfg *config.Config, specPath, params string, quiet bool) error {
	// Load DAG
	dag, err := digraph.Load(ctx, cfg.Paths.BaseConfig, specPath, params)
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", specPath, err)
	}

	// Generate request ID
	requestID, err := generateRequestID()
	if err != nil {
		return fmt.Errorf("failed to generate request ID: %w", err)
	}

	// Setup logging
	logFile, err := openLogFile(logFileSettings{
		Prefix:    startPrefix,
		LogDir:    cfg.Paths.LogDir,
		DAGLogDir: dag.LogDir,
		DAGName:   dag.Name,
		RequestID: requestID,
	})
	if err != nil {
		return fmt.Errorf("failed to create log file for DAG %s: %w", dag.Name, err)
	}
	defer logFile.Close()

	// Initialize services
	dataStore := newDataStores(cfg)
	cli := newClient(cfg, dataStore)

	logger.Info(ctx, "DAG execution initiated", "DAG", dag.Name, "requestID", requestID, "logFile", logFile.Name())
	ctx = logger.WithLogger(ctx, buildLoggerWithFile(logFile, quiet))

	// Create and run agent
	agt := agent.New(
		requestID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		cli,
		dataStore,
		&agent.Options{},
	)

	listenSignals(ctx, agt)

	if err := agt.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute DAG %s (requestID: %s): %w", dag.Name, requestID, err)
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
