package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/frontend"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		// nolint:line-length-limit
		Long: `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
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

			app := fx.New(
				frontendModule,
				fx.Provide(func() *config.Config { return cfg }),
				fx.Invoke(frontend.LifetimeHooks),
			)

			if err := app.Start(cmd.Context()); err != nil {
				// nolint
				log.Fatalf("Failed to start server: %v", err)
			}
		},
	}

	bindServerCommandFlags(cmd)
	return cmd
}

func bindServerCommandFlags(cmd *cobra.Command) {
	cmd.Flags().StringP(
		"dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)",
	)
	cmd.Flags().StringP("host", "s", "", "server host (default is localhost)")
	cmd.Flags().StringP("port", "p", "", "server port (default is 8080)")
}
