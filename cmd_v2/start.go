package cmd_v2

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/dag"
)

var startCommand = &cobra.Command{
	Use:   "start [flags] <YAML file>",
	Short: "Runs specified DAG",
	Long:  `dagu start [--params="param1 param2"] <YAML file>`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]
		params, err := cmd.Flags().GetString("params")
		if err != nil {
			panic(err)
		}
		d, err := loadDAG(file, strings.Trim(params, `"`))
		if err != nil {
			panic(err)
		}
		if err := start(d); err != nil {
			panic(err)
		}
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
