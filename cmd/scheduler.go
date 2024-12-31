// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func schedulerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		RunE:  wrapRunE(runScheduler),
	}

	cmd.Flags().StringP(
		"dags",
		"d",
		"",
		"location of DAG files (default is $HOME/.config/dagu/dags)",
	)
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}

func runScheduler(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ctx := cmd.Context()
	ctx = logger.WithLogger(ctx, buildLogger(cfg, false))

	// Update DAGs directory if specified
	if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
		cfg.Paths.DAGsDir = dagsDir
	}

	logger.Info(ctx, "Scheduler initialization",
		"specsDirectory", cfg.Paths.DAGsDir,
		"logFormat", cfg.LogFormat)

	dataStore := newDataStores(cfg)
	dagStore := newDAGStore(cfg)
	cli := newClient(cfg, dataStore, dagStore)

	sc := scheduler.New(cfg, cli)
	if err := sc.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler in directory %s: %w",
			cfg.Paths.DAGsDir, err)
	}

	return nil
}
