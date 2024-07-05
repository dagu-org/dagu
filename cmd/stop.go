package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <DAG file>",
		Short: "Stop the running DAG",
		Long:  `dagu stop <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}

			loadedDAG, err := loadDAG(cfg, args[0], "")
			if err != nil {
				log.Fatalf("Failed to load DAG: %v", err)
			}

			log.Printf("Stopping...")

			eng := newEngine(cfg)

			if err := eng.Stop(loadedDAG); err != nil {
				log.Fatalf("Failed to stop the DAG: %v", err)
			}
		},
	}
}
