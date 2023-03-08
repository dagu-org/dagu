package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/runner"
)

func schedulerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg.DAGs = getFlagString(cmd, "dags", cfg.DAGs)
			agent := runner.NewAgent(cfg)
			listenSignals(func(sig os.Signal) { agent.Stop() })
			cobra.CheckErr(agent.Start())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "DAGs dir")
	return cmd
}
