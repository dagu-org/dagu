package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <DAG file>",
		Short: "Stop the running DAG",
		Long:  `dagu stop <DAG file>`,
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(config.LoadConfig())
		},
		Run: func(cmd *cobra.Command, args []string) {
			loadedDAG, err := loadDAG(args[0], "")
			if err != nil {
				log.Fatalf("Failed to load DAG: %v", err)
			}

			log.Printf("Stopping...")

			if err := engine.New(
				client.NewDataStoreFactory(config.Get()),
				engine.DefaultConfig(),
				config.Get(),
			).Stop(loadedDAG); err != nil {
				log.Fatalf("Failed to stop the DAG: %v", err)
			}
		},
	}
}
