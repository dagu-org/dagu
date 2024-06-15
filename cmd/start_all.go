package cmd

import (
	"log"

	scheduler "github.com/dagu-dev/dagu/service"
	"github.com/dagu-dev/dagu/service/frontend"
	"go.uber.org/fx"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func startAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-all",
		Short: "Launches both the Dagu web UI server and the scheduler process.",
		Long:  `dagu start-all [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		PreRun: func(cmd *cobra.Command, args []string) {
			_ = viper.BindPFlag("port", cmd.Flags().Lookup("port"))
			_ = viper.BindPFlag("host", cmd.Flags().Lookup("host"))
			_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}

			if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
				cfg.DAGs = dagsDir
			}

			opts := fx.Options(
				fx.Provide(func() *config.Config { return cfg }),
				baseModule,
			)
			dagScheduler := scheduler.New(opts)

			// Start the scheduler process.
			ctx := cmd.Context()
			go func() {
				err := dagScheduler.Start(ctx)
				if err != nil {
					log.Fatal(err) // nolint // deep-exit
				}
			}()

			app := fx.New(
				frontendModule,
				fx.Provide(func() *config.Config { return cfg }),
				fx.Invoke(frontend.LifetimeHooks),
			)

			if err := app.Start(ctx); err != nil {
				log.Fatalf("Failed to start server: %v", err)
			}
		},
	}

	bindStartAllCommandFlags(cmd)
	return cmd
}

func bindStartAllCommandFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	cmd.Flags().StringP("host", "s", "", "server host (default is localhost)")
	cmd.Flags().StringP("port", "p", "", "server port (default is 8080)")
}
