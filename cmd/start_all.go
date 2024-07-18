package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/frontend"
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
				err := scheduler.Start(ctx)
				if err != nil {
					log.Fatal(err) // nolint // deep-exit
				}
			}()

			// Start the frontend server.
			frontend := fx.New(
				frontendModule,
				fx.Provide(func() *config.Config { return cfg }),
				fx.Invoke(frontend.LifetimeHooks),
			)

			if err := frontend.Start(ctx); err != nil {
				log.Fatalf("Failed to start server: %v", err)
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
