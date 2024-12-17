// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"log"

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
		Run: func(cmd *cobra.Command, _ []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Configuration load failed: %v", err)
			}
			logger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:  cfg.Debug,
				Format: cfg.LogFormat,
			})

			if dagsOpt, _ := cmd.Flags().GetString("dags"); dagsOpt != "" {
				cfg.DAGs = dagsOpt
			}

			logger.Info("Scheduler initialization",
				"specsDirectory", cfg.DAGs,
				"logFormat", cfg.LogFormat)

			ctx := cmd.Context()
			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, logger)
			sc := scheduler.New(cfg, logger, cli)
			if err := sc.Start(ctx); err != nil {
				logger.Fatal(
					"Scheduler initialization failed",
					"error",
					err,
					"specsDirectory",
					cfg.DAGs,
				)
			}
		},
	}

	cmd.Flags().StringP(
		"dags", "d", "", "location of DAG files (default is $HOME/.config/dagu/dags)",
	)
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}
