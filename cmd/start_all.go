package cmd

import (
	"log"
	"os"

	"github.com/daguflow/dagu/internal/config"
	"github.com/daguflow/dagu/internal/frontend"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/scheduler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func startAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "start-all",
		// nolint
		Short: "Launches both the Dagu web UI server and the scheduler process.",
		// nolint
		Long: `dagu start-all [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		PreRun: func(cmd *cobra.Command, _ []string) {
			_ = viper.BindPFlag("port", cmd.Flags().Lookup("port"))
			_ = viper.BindPFlag("host", cmd.Flags().Lookup("host"))
			_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))
		},
		Run: func(cmd *cobra.Command, _ []string) {
			cfg, err := config.Load()
			if err != nil {
				// nolint
				log.Fatalf("Configuration load failed: %v", err)
			}
			logger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
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
					logger.Error("Scheduler initialization failed", "error", err, "dags", cfg.DAGs)
					os.Exit(1)
				}
			}()

			logger.Info("Server initialization", "host", cfg.Host, "port", cfg.Port)

			server := frontend.New(cfg, logger, cli)
			if err := server.Serve(ctx); err != nil {
				logger.Error("Server initialization failed", "error", err)
				os.Exit(1)
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
