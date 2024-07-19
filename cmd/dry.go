package cmd

import (
	"log"
	"os"
	"path/filepath"

	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func dryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] <DAG file>",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				// nolint
				log.Fatalf("Failed to load config: %v", err)
			}
			initLogger := logger.NewLogger(logger.NewLoggerArgs{
				Config: cfg,
			})

			params, err := cmd.Flags().GetString("params")
			if err != nil {
				initLogger.Error("Failed to get params", "error", err)
				os.Exit(1)
			}

			dg, err := dag.Load(cfg.BaseConfig, args[0], params)
			if err != nil {
				initLogger.Error("Failed to load DAG", "error", err)
				os.Exit(1)
			}

			eng := newEngine(cfg)

			requestID, err := generateRequestID()
			if err != nil {
				initLogger.Error("Failed to generate request ID", "error", err)
				os.Exit(1)
			}

			logFile, err := openLogFileForDAG("dry_", cfg.LogDir, dg, requestID)
			if err != nil {
				initLogger.Error("Failed to open log file for DAG", "error", err)
				os.Exit(1)
			}
			defer logFile.Close()

			fileLogger := logger.NewLogger(logger.NewLoggerArgs{
				Config:  cfg,
				LogFile: logFile,
			})

			dagAgent := agent.New(
				requestID,
				dg,
				fileLogger,
				filepath.Dir(logFile.Name()),
				logFile.Name(),
				eng,
				newDataStores(cfg),
				&agent.AgentOpts{Dry: true})

			ctx := cmd.Context()

			listenSignals(ctx, dagAgent)

			if err := dagAgent.Run(ctx); err != nil {
				fileLogger.Error("Failed to start DAG", "error", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}
