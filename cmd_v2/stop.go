package cmd_v2

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
)

func stopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <DAG file>",
		Short: "Stop specified DAG",
		Long:  `dagu stop <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			d, err := loadDAG(args[0], "")
			cobra.CheckErr(err)
			cobra.CheckErr(stop(d))
		},
	}
}

func stop(d *dag.DAG) error {
	log.Printf("Stopping...")
	return controller.NewDAGController(d).Stop()
}
