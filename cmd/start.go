package main

import (
	"os"
	"path/filepath"

	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func startCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] -n <workflow name>",
		Short: "Runs the workflow",
		Long:  `dagu start [--params="param1 param2"] <workflow name>`,
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			initialize(cmd)

			name := cmd.Flag("name").Value.String()
			params := cmd.Flag("params").Value.String()

			workflow, err := dag.Load(appConfig.BaseConfig, name, params)
			if err != nil {
				appLogger.Error("Workflow load failed", "error", err, "file", args[0])
				os.Exit(1)
			}

			requestID, err := generateRequestID()
			if err != nil {
				appLogger.Error("Request ID generation failed", "error", err)
				os.Exit(1)
			}

			logFile, err := openLogFile("start_", appConfig.LogDir, workflow, requestID)
			if err != nil {
				appLogger.Error(
					"Log file creation failed",
					"error",
					err,
					"workflow",
					workflow.Name,
				)
				os.Exit(1)
			}
			defer logFile.Close()

			agentLogger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  appConfig.LogLevel,
				LogFormat: appConfig.LogFormat,
				LogFile:   logFile,
				Quiet:     quiet,
			})

			dataStore := newDataStores(appConfig)
			cli := newClient(appConfig, dataStore, agentLogger)

			agentLogger.Info("Workflow execution initiated",
				"workflow", workflow.Name,
				"requestID", requestID,
				"logFile", logFile.Name())

			agt := agent.New(
				requestID,
				workflow,
				agentLogger,
				filepath.Dir(logFile.Name()),
				logFile.Name(),
				cli,
				dataStore,
				&agent.Options{})

			ctx := cmd.Context()

			listenSignals(ctx, agt)

			if err := agt.Run(ctx); err != nil {
				agentLogger.Error("Workflow execution failed",
					"error", err,
					"workflow", workflow.Name,
					"requestID", requestID)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringP("params", "p", "", "parameters")
	cmd.Flags().StringP("name", "n", "", "workflow name")
	if err := cmd.MarkFlagRequired("name"); err != nil {
		appLogger.Error("Flag marking failed", "error", err)
		os.Exit(1)
	}

	return cmd
}
