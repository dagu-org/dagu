// Copyright (C) 2024 The Daguflow/Dagu Authors
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
	"os"
	"path/filepath"

	"github.com/daguflow/dagu/internal/agent"
	"github.com/daguflow/dagu/internal/config"
	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func dryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] /path/to/spec.yaml",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				// nolint
				log.Fatalf("Failed to load config: %v", err)
			}
			initLogger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
			})

			params, err := cmd.Flags().GetString("params")
			if err != nil {
				initLogger.Error("Parameter retrieval failed", "error", err)
				os.Exit(1)
			}

			workflow, err := dag.Load(cfg.BaseConfig, args[0], removeQuotes(params))
			if err != nil {
				initLogger.Error("Workflow load failed", "error", err, "file", args[0])
				os.Exit(1)
			}

			requestID, err := generateRequestID()
			if err != nil {
				initLogger.Error("Request ID generation failed", "error", err)
				os.Exit(1)
			}

			logFile, err := openLogFile("dry_", cfg.LogDir, workflow, requestID)
			if err != nil {
				initLogger.Error(
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
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
				LogFile:   logFile,
			})

			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, agentLogger)

			agt := agent.New(
				requestID,
				workflow,
				agentLogger,
				filepath.Dir(logFile.Name()),
				logFile.Name(),
				cli,
				dataStore,
				&agent.Options{Dry: true})

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
	return cmd
}
