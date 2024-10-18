// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cmd

import (
	"log"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] /path/to/spec.yaml",
		Short: "Runs the DAG",
		Long:  `dagu start [--params="param1 param2"] /path/to/spec.yaml`,
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
			waiting, err := cmd.Flags().GetBool("waiting")
			if err != nil {
				log.Fatalf("Flag retrieval failed (waiting): %v", err)
			}

			initLogger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:  cfg.Debug,
				Format: cfg.LogFormat,
				Quiet:  quiet,
			})

			params, err := cmd.Flags().GetString("params")
			if err != nil {
				initLogger.Fatal("Parameter retrieval failed", "error", err)
			}

			workflow, err := dag.Load(cfg.BaseConfig, args[0], removeQuotes(params))
			if err != nil {
				initLogger.Fatal("Workflow load failed", "error", err, "file", args[0])
			}

			requestID, err := generateRequestID()
			if err != nil {
				initLogger.Fatal("Request ID generation failed", "error", err)
			}

			logFile, err := logger.OpenLogFile(logger.LogFileConfig{
				Prefix:    "start_",
				LogDir:    cfg.LogDir,
				DAGLogDir: workflow.LogDir,
				DAGName:   workflow.Name,
				RequestID: requestID,
			})
			if err != nil {
				initLogger.Fatal(
					"Log file creation failed",
					"error",
					err,
					"workflow",
					workflow.Name,
				)
			}
			defer logFile.Close()

			agentLogger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:   cfg.Debug,
				Format:  cfg.LogFormat,
				LogFile: logFile,
				Quiet:   quiet,
			})

			dataStore := newDataStores(cfg)

			queueStore := newQueueStore(cfg)
			statsStore := newStatsStore(cfg)

			cli := newClient(cfg, dataStore, agentLogger)

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
				cfg.DAGQueueLength,
				cli,
				dataStore,
				queueStore,
				statsStore,
				&agent.Options{FromWaitingQueue: waiting})
			ctx := cmd.Context()

			listenSignals(ctx, agt)

			if err := agt.Run(ctx); err != nil {
				agentLogger.Fatal("Workflow execution failed",
					"error", err,
					"workflow", workflow.Name,
					"requestID", requestID)
			}
		},
	}

	cmd.Flags().StringP("params", "p", "", "parameters")
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	cmd.Flags().BoolP("waiting", "w", false, "from waiting queue")

	return cmd
}

// removeQuotes removes the surrounding quotes from the string.
func removeQuotes(s string) string {
	if len(s) > 1 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
