package cmd_v2

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
)

var stopCommand = &cobra.Command{
	Use:   "stop <YAML file>",
	Short: "Stop specified DAG",
	Long:  `dagu stop <YAML file>`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]
		d, err := loadDAG(file, "")
		cobra.CheckErr(err)
		cobra.CheckErr(stop(d))
	},
}

func stop(d *dag.DAG) error {
	c := controller.NewDAGController(d)
	log.Printf("Stopping...")
	return c.Stop()
}
