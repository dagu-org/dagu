package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <DAG file>",
		Short: "Display current status of the DAG",
		Long:  `dagu status <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
			// Load the DAG file and get the current running status.
			loadedDAG, err := loadDAG(cfg, args[0], "")
			if err != nil {
				log.Fatalf("Failed to load DAG: %v", err)
			}

			curStatus, err := engine.New(
				client.NewDataStoreFactory(cfg),
				engine.DefaultConfig(),
				cfg,
			).GetCurrentStatus(loadedDAG)

			if err != nil {
				log.Fatalf("Failed to get the current status: %v", err)
			}

			log.Printf("Pid=%d Status=%s", curStatus.Pid, curStatus.Status)
		},
	}
}
