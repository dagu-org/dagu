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

func retryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry --req=<request-id> /path/to/spec.yaml",
		Short: "Retry the DAG execution",
		Long:  `dagu retry --req=<request-id> /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
			initLogger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
			})

			reqID, err := cmd.Flags().GetString("req")
			if err != nil {
				initLogger.Error("Request ID is required", "error", err)
				os.Exit(1)
			}

			// Read the specified DAG execution status from the history store.
			dataStore := newDataStores(cfg)
			historyStore := dataStore.HistoryStore()

			absoluteFilePath, err := filepath.Abs(args[0])
			if err != nil {
				initLogger.Error("Failed to get the absolute path of the DAG file", "error", err)
				os.Exit(1)
			}

			status, err := historyStore.FindByRequestID(absoluteFilePath, reqID)
			if err != nil {
				initLogger.Error("Failed to find the request", "error", err)
				os.Exit(1)
			}

			// Start the DAG with the same parameters with the execution that
			// is being retried.
			workflow, err := dag.Load(cfg.BaseConfig, args[0], status.Status.Params)
			if err != nil {
				initLogger.Error("Failed to load DAG", "error", err)
				os.Exit(1)
			}

			requestID, err := generateRequestID()
			if err != nil {
				initLogger.Error("Failed to generate request ID", "error", err)
				os.Exit(1)
			}

			logFile, err := openLogFile("dry_", cfg.LogDir, workflow, requestID)
			if err != nil {
				initLogger.Error("Failed to open log file for DAG", "error", err)
				os.Exit(1)
			}
			defer logFile.Close()

			agentLogger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
				LogFile:   logFile,
			})

			ds := newDataStores(cfg)
			eng := newEngine(cfg, ds, agentLogger)

			agentLogger.Infof("Retrying with request ID: %s", requestID)

			dagAgent := agent.New(
				requestID,
				workflow,
				agentLogger,
				filepath.Dir(logFile.Name()),
				logFile.Name(),
				eng,
				newDataStores(cfg),
				&agent.AgentOpts{RetryTarget: status.Status},
			)

			ctx := cmd.Context()
			listenSignals(ctx, dagAgent)

			if err := dagAgent.Run(ctx); err != nil {
				agentLogger.Error("Failed to start workflow", "error", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringP("req", "r", "", "request-id")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}
