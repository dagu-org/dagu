package cmd

import (
	"log"
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"
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
			logger := logger.NewLogger(logger.NewLoggerArgs{
				Config: cfg,
			})

			// Load the DAG file and get the current running status.
			workflow, err := dag.Load(cfg.BaseConfig, args[0], "")
			if err != nil {
				// nolint
				logger.Error("Failed to load DAG", "error", err)
				os.Exit(1)
			}

			ds := newDataStores(cfg)
			eng := newEngine(cfg, ds)

			curStatus, err := eng.GetCurrentStatus(workflow)

			if err != nil {
				// nolint
				logger.Error("Failed to get the current status", "error", err)
				os.Exit(1)
			}

			logger.Info("Current status", "pid", curStatus.PID, "status", curStatus.Status)
		},
	}
}
