package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	scheduler "github.com/dagu-dev/dagu/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

func schedulerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
			if dagsOpt, _ := cmd.Flags().GetString("dags"); dagsOpt != "" {
				cfg.DAGs = dagsOpt
			}
			opts := fx.Options(
				fx.Provide(func() *config.Config { return cfg }),
				baseModule,
			)
			if err := scheduler.New(opts).Start(cmd.Context()); err != nil {
				log.Fatalf("Failed to start scheduler: %v", err)
			}
		},
	}

	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}
