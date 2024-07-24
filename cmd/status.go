package cmd

import (
	"log"
	"os"

	"github.com/daguflow/dagu/internal/config"
	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status /path/to/spec.yaml",
		Short: "Display current status of the DAG",
		Long:  `dagu status /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				// nolint
				log.Fatalf("Configuration load failed: %v", err)
			}
			logger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
			})

			// Load the DAG file and get the current running status.
			workflow, err := dag.Load(cfg.BaseConfig, args[0], "")
			if err != nil {
				// nolint
				logger.Error("Workflow load failed", "error", err, "file", args[0])
				os.Exit(1)
			}

			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, logger)

			curStatus, err := cli.GetCurrentStatus(workflow)

			if err != nil {
				// nolint
				logger.Error("Current status retrieval failed", "error", err)
				os.Exit(1)
			}

			logger.Info("Current status", "pid", curStatus.PID, "status", curStatus.Status)
		},
	}
}
