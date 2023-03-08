package cmd_v2

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/dag"
)

func startCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] <DAG file>",
		Short: "Runs the DAG",
		Long:  `dagu start [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			params, err := cmd.Flags().GetString("params")
			cobra.CheckErr(err)
			d, err := loadDAG(args[0], strings.Trim(params, `"`))
			cobra.CheckErr(err)
			cobra.CheckErr(start(d, false))
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}

func start(d *dag.DAG, dry bool) error {
	a := &agent.Agent{AgentConfig: &agent.AgentConfig{DAG: d, Dry: dry}}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}