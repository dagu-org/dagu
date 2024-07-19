package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <DAG file>",
		Short: "Display current status of the DAG",
		Long:  `dagu status <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				// nolint
				log.Fatalf("Failed to load config: %v", err)
			}

			// Load the DAG file and get the current running status.
			loadedDAG, err := dag.Load(cfg.BaseConfig, args[0], "")
			if err != nil {
				// nolint
				log.Fatalf("Failed to load DAG: %v", err)
			}

			eng := newEngine(cfg)

			curStatus, err := eng.GetCurrentStatus(loadedDAG)

			if err != nil {
				// nolint
				log.Fatalf("Failed to get the current status: %v", err)
			}

			log.Printf("Pid=%d Status=%s", curStatus.PID, curStatus.Status)
		},
	}
}
