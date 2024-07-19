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

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] <DAG file>",
		Short: "Runs the DAG",
		Long:  `dagu start [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				// nolint
				log.Fatalf("Failed to load config: %v", err)
			}

			quiet, err := cmd.Flags().GetBool("quiet")
			if err != nil {
				log.Fatalf("Failed to get quiet flag: %v", err)
			}

			initLogger := logger.NewLogger(logger.NewLoggerArgs{
				Config: cfg,
				Quiet:  quiet,
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

			logFile, err := openLogFileForDAG("start_", cfg.LogDir, dg, requestID)
			if err != nil {
				initLogger.Error("Failed to open log file for DAG", "error", err)
				os.Exit(1)
			}
			defer logFile.Close()

			agentLogger := logger.NewLogger(logger.NewLoggerArgs{
				Config:  cfg,
				LogFile: logFile,
				Quiet:   quiet,
			})

			agentLogger.Info("Starting DAG", "dag", dg.Name)

			dagAgent := agent.New(
				requestID,
				dg,
				agentLogger,
				filepath.Dir(logFile.Name()),
				logFile.Name(),
				eng,
				newDataStores(cfg),
				&agent.AgentOpts{})

			ctx := cmd.Context()

			listenSignals(ctx, dagAgent)

			if err := dagAgent.Run(ctx); err != nil {
				agentLogger.Error("Failed to start DAG", "error", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringP("params", "p", "", "parameters")
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}
