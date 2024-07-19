package cmd

import (
	"log"
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/frontend"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
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
				log.Fatalf("Failed to load config: %v", err)
			}
			logger := logger.NewLogger(logger.NewLoggerArgs{
				Config: cfg,
			})

			if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
				cfg.DAGs = dagsDir
			}

			ctx := cmd.Context()

			// Start the scheduler process.
			scheduler := fx.New(
				schedulerModule,
				fx.Provide(func() *config.Config { return cfg }),
				fx.Invoke(scheduler.LifetimeHooks),
			)

			go func() {
				logger.Info("Starting the scheduler", "dags", cfg.DAGs)
				err := scheduler.Start(ctx)
				if err != nil {
					logger.Error("Failed to start scheduler", "error", err)
					os.Exit(1)
				}
			}()

			// Start the frontend server.
			frontend := fx.New(
				frontendModule,
				fx.Provide(func() *config.Config { return cfg }),
				fx.Invoke(frontend.LifetimeHooks),
			)

			logger.Info("Starting the server", "host", cfg.Host, "port", cfg.Port)

			if err := frontend.Start(ctx); err != nil {
				logger.Error("Failed to start server", "error", err)
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
