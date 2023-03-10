package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/runner"
)

func schedulerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			config.Get().DAGs = getFlagString(cmd, "dags", config.Get().DAGs)
			agent := runner.NewAgent(config.Get())
			listenSignals(func(sig os.Signal) { agent.Stop() })
			cobra.CheckErr(agent.Start())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}
