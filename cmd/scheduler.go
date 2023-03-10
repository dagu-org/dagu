package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/runner"
)

func schedulerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			config.C.DAGs = getFlagString(cmd, "dags", config.C.DAGs)
			agent := runner.NewAgent(config.C)
			listenSignals(func(sig os.Signal) { agent.Stop() })
			cobra.CheckErr(agent.Start())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	return cmd
}
