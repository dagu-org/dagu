package cmd

import (
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/spf13/cobra"
	"log"
)

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <DAG file>",
		Short: "Stop the running DAG",
		Long:  `dagu stop <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			loadedDAG, err := loadDAG(args[0], "")
			checkError(err)

			log.Printf("Stopping...")

			// TODO: fix this
			e := engine.NewFactory().Create()
			checkError(e.Stop(loadedDAG))
		},
	}
}
