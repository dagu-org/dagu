package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/runner"
)

func createSchedulerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			config.Get().DAGs = getFlagString(cmd, "dags", config.Get().DAGs)
			agent := runner.NewAgent(config.Get())
			listenSignals(cmd.Context(), agent)
			checkError(agent.Start())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}
