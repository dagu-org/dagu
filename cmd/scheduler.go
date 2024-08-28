// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cmd

import (
	"log"

	"github.com/daguflow/dagu/internal/config"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/scheduler"
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
