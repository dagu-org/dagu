package cmd

import (
	"log"
	"os"

	"github.com/daguflow/dagu/internal/config"
	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/logger"
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
				log.Fatalf("Configuration load failed: %v", err)
			}

			quiet, err := cmd.Flags().GetBool("quiet")
			if err != nil {
				log.Fatalf("Flag retrieval failed (quiet): %v", err)
			}

			logger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
				Quiet:     quiet,
			})

			workflow, err := dag.Load(cfg.BaseConfig, args[0], "")
			if err != nil {
				logger.Error("Workflow load failed", "error", err, "file", args[0])
				os.Exit(1)
			}

			logger.Info("Workflow stop initiated", "workflow", workflow.Name)

			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, logger)

			if err := cli.Stop(workflow); err != nil {
				logger.Error(
					"Workflow stop operation failed",
					"error",
					err,
					"workflow",
					workflow.Name,
				)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}
