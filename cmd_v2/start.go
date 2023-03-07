package cmd_v2

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/dag"
)

var startCommand = &cobra.Command{
	Use:   "start [flags] <DAG file>",
	Short: "Runs specified DAG",
	Long:  `dagu start [--params="param1 param2"] <DAG file>`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		params, err := cmd.Flags().GetString("params")
		cobra.CheckErr(err)
		d, err := loadDAG(args[0], strings.Trim(params, `"`))
		cobra.CheckErr(err)
		cobra.CheckErr(start(d))
	},
}

func start(d *dag.DAG) error {
	a := &agent.Agent{AgentConfig: &agent.AgentConfig{
		DAG: d,
		Dry: false,
	}}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}
