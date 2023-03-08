package cmd_v2

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/controller"
)

func stopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <DAG file>",
		Short: "Stop the running DAG",
		Long:  `dagu stop <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			d, err := loadDAG(args[0], "")
			cobra.CheckErr(err)

			log.Printf("Stopping...")
			cobra.CheckErr(controller.NewDAGController(d).Stop())
		},
	}
}
