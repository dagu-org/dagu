// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
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
				log.Fatalf("Configuration load failed: %v", err)
			}
			initLogger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:  cfg.Debug,
				Format: cfg.LogFormat,
			})

			requestID, err := cmd.Flags().GetString("req")
			if err != nil {
				initLogger.Error("Request ID generation failed", "error", err)
				os.Exit(1)
			}

			// Read the specified DAG execution status from the history store.
			dataStore := newDataStores(cfg)
			historyStore := dataStore.HistoryStore()

			specFilePath := args[0]
			absoluteFilePath, err := filepath.Abs(specFilePath)
			if err != nil {
				initLogger.Fatal("Absolute path resolution failed",
					"error", err,
					"file", specFilePath)
			}

			ctx := cmd.Context()
			status, err := historyStore.FindByRequestID(ctx, absoluteFilePath, requestID)
			if err != nil {
				initLogger.Fatal("Historical execution retrieval failed",
					"error", err,
					"requestID", requestID,
					"file", absoluteFilePath)
			}

			// Start the DAG with the same parameters with the execution that
			// is being retried.
			dag, err := digraph.Load(ctx, cfg.BaseConfig, absoluteFilePath, status.Status.Params)
			if err != nil {
				initLogger.Fatal("DAG specification load failed",
					"error", err,
					"file", specFilePath,
					"params", status.Status.Params)
			}

			newRequestID, err := generateRequestID()
			if err != nil {
				initLogger.Fatal("Request ID generation failed", "error", err)
			}

			logFile, err := logger.OpenLogFile(logger.LogFileConfig{
				Prefix:    "retry_",
				LogDir:    cfg.LogDir,
				DAGLogDir: dag.LogDir,
				DAGName:   dag.Name,
				RequestID: newRequestID,
			})
			if err != nil {
				initLogger.Fatal("Log file creation failed",
					"error", err,
					"DAG", dag.Name)
			}
			defer logFile.Close()

			agentLogger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:   cfg.Debug,
				Format:  cfg.LogFormat,
				LogFile: logFile,
			})

			cli := newClient(cfg, dataStore, agentLogger)

			agentLogger.Info("DAG retry initiated",
				"DAG", dag.Name,
				"originalRequestID", requestID,
				"newRequestID", newRequestID,
				"logFile", logFile.Name())

			agt := agent.New(
				newRequestID,
				dag,
				agentLogger,
				filepath.Dir(logFile.Name()),
				logFile.Name(),
				cli,
				dataStore,
				&agent.Options{RetryTarget: status.Status},
			)

			listenSignals(ctx, agt)

			if err := agt.Run(ctx); err != nil {
				agentLogger.Fatal("Failed to start DAG", "error", err)
			}
		},
	}

	cmd.Flags().StringP("req", "r", "", "request-id")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}
