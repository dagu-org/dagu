package cmd

import (
	"github.com/dagu-dev/dagu/app"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func createSchedulerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu backend [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			// TODO: fixme
			config.Get().DAGs = getFlagString(cmd, "dags", config.Get().DAGs)

			service := app.NewSchedulerService()
			err := service.Start(cmd.Context())
			checkError(err)
		},
	}
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}
