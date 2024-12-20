// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status /path/to/spec.yaml",
		Short: "Display current status of the DAG",
		Long:  `dagu status /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ctx := cmd.Context()
	ctx = logger.WithLogger(ctx, buildLogger(cfg))

	// Load the DAG
	dag, err := digraph.Load(ctx, cfg.BaseConfig, args[0], "")
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	// Initialize services and get status
	dataStore := newDataStores(cfg)
	cli := newClient(cfg, dataStore)

	status, err := cli.GetCurrentStatus(ctx, dag)
	if err != nil {
		return fmt.Errorf("failed to retrieve current status: %w", err)
	}

	// Log the status information
	logger.Info(ctx, "Current status", "pid", status.PID, "status", status.Status)

	return nil
}
