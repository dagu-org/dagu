package cmd

import (
	"log"
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop /path/to/spec.yaml",
		Short: "Stop the running workflow",
		Long:  `dagu stop /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}

			quiet, err := cmd.Flags().GetBool("quiet")
			if err != nil {
				log.Fatalf("Failed to get quiet flag: %v", err)
			}

			logger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
				Quiet:     quiet,
			})

			workflow, err := dag.Load(cfg.BaseConfig, args[0], "")
			if err != nil {
				logger.Error("Failed to load workflow", "error", err)
				os.Exit(1)
			}

			logger.Infof("Stopping workflow: %s", workflow.Name)

			dataStore := newDataStores(cfg)
			eng := newEngine(cfg, dataStore, logger)

			if err := eng.Stop(workflow); err != nil {
				logger.Error("Failed to stop the workflow", "error", err)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}
