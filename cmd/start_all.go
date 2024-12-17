// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"log"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/scheduler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func startAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-all",
		Short: "Launches both the Dagu web UI server and the scheduler process.",
		Long:  `dagu start-all [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			_ = viper.BindPFlag("port", cmd.Flags().Lookup("port"))
			_ = viper.BindPFlag("host", cmd.Flags().Lookup("host"))
			_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))
		},
		Run: func(cmd *cobra.Command, _ []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Configuration load failed: %v", err)
			}
			logger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:  cfg.Debug,
				Format: cfg.LogFormat,
			})

			if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
				cfg.DAGs = dagsDir
			}

			ctx := cmd.Context()
			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, logger)

			go func() {
				logger.Info("Scheduler initialization", "dags", cfg.DAGs)

				sc := scheduler.New(cfg, logger, cli)
				if err := sc.Start(ctx); err != nil {
					logger.Fatal("Scheduler initialization failed", "error", err, "dags", cfg.DAGs)
				}
			}()

			logger.Info("Server initialization", "host", cfg.Host, "port", cfg.Port)

			server := frontend.New(cfg, logger, cli)
			if err := server.Serve(ctx); err != nil {
				logger.Fatal("Server initialization failed", "error", err)
			}
		},
	}

	bindStartAllCommandFlags(cmd)
	return cmd
}

func bindStartAllCommandFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(
		"dags", "d", "", "location of DAG files (default is $HOME/.config/dagu/dags)",
	)
	cmd.Flags().StringP("host", "s", "", "server host (default is localhost)")
	cmd.Flags().StringP("port", "p", "", "server port (default is 8080)")
}
