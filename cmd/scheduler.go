package cmd

import (
	"log"
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/scheduler"
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
				// nolint
				log.Fatalf("Failed to load config: %v", err)
			}
			logger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
			})

			if dagsOpt, _ := cmd.Flags().GetString("dags"); dagsOpt != "" {
				cfg.DAGs = dagsOpt
			}

			logger.Info("Starting the scheduler", "dags", cfg.DAGs)

			ctx := cmd.Context()
			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, logger)
			sc := scheduler.New(cfg, logger, cli)
			if err := sc.Start(ctx); err != nil {
				logger.Error("Failed to start scheduler", "error", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringP(
		"dags", "d", "", "location of DAG files (default is $HOME/.config/dagu/dags)",
	)
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}
