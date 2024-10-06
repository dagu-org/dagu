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
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func restartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart /path/to/spec.yaml",
		Short: "Stop the running DAG and restart it",
		Long:  `dagu restart /path/to/spec.yaml`,
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

			initLogger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:  cfg.Debug,
				Format: cfg.LogFormat,
				Quiet:  quiet,
			})

			// Load the DAG file and stop the DAG if it is running.
			specFilePath := args[0]
			workflow, err := dag.Load(cfg.BaseConfig, specFilePath, "")
			if err != nil {
				initLogger.Fatal("Workflow load failed", "error", err, "file", args[0])
			}

			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, initLogger)
			queueStore := newQueueStore(cfg)
			statsStore := newStatsStore(cfg)

			if err := stopDAGIfRunning(cli, workflow, initLogger); err != nil {
				initLogger.Fatal("Workflow stop operation failed",
					"error", err,
					"workflow", workflow.Name)
			}

			// Wait for the specified amount of time before restarting.
			waitForRestart(workflow.RestartWait, initLogger)

			// Retrieve the parameter of the previous execution.
			params, err := getPreviousExecutionParams(cli, workflow)
			if err != nil {
				initLogger.Fatal("Previous execution parameter retrieval failed",
					"error", err,
					"workflow", workflow.Name)
			}

			// Start the DAG with the same parameter.
			// Need to reload the DAG file with the parameter.
			workflow, err = dag.Load(cfg.BaseConfig, specFilePath, params)
			if err != nil {
				initLogger.Fatal("Workflow reload failed",
					"error", err,
					"file", specFilePath,
					"params", params)
			}

			requestID, err := generateRequestID()
			if err != nil {
				initLogger.Fatal("Request ID generation failed", "error", err)
			}

			logFile, err := logger.OpenLogFile(logger.LogFileConfig{
				Prefix:    "restart_",
				LogDir:    cfg.LogDir,
				DAGLogDir: workflow.LogDir,
				DAGName:   workflow.Name,
				RequestID: requestID,
			})
			if err != nil {
				initLogger.Fatal("Log file creation failed",
					"error", err,
					"workflow", workflow.Name)
			}
			defer logFile.Close()

			agentLogger := logger.NewLogger(logger.NewLoggerArgs{
				Debug:   cfg.Debug,
				Format:  cfg.LogFormat,
				LogFile: logFile,
				Quiet:   quiet,
			})

			agentLogger.Info("Workflow restart initiated",
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

				newClient(cfg, dataStore, agentLogger),
				dataStore,
				queueStore,
				statsStore,
				&agent.Options{Dry: false})

			listenSignals(cmd.Context(), agt)
			if err := agt.Run(cmd.Context()); err != nil {
				agentLogger.Fatal("Workflow restart failed",
					"error", err,
					"workflow", workflow.Name,
					"requestID", requestID)
			}
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}

// stopDAGIfRunning stops the DAG if it is running.
// Otherwise, it does nothing.
func stopDAGIfRunning(e client.Client, workflow *dag.DAG, lg logger.Logger) error {
	curStatus, err := e.GetCurrentStatus(workflow)
	if err != nil {
		return err
	}

	if curStatus.Status == scheduler.StatusRunning {
		lg.Infof("Stopping: %s", workflow.Name)
		cobra.CheckErr(stopRunningDAG(e, workflow))
	}
	return nil
}

// stopRunningDAG attempts to stop the running DAG
// by sending a stop signal to the agent.
func stopRunningDAG(e client.Client, workflow *dag.DAG) error {
	for {
		curStatus, err := e.GetCurrentStatus(workflow)
		if err != nil {
			return err
		}

		// If the DAG is not running, do nothing.
		if curStatus.Status != scheduler.StatusRunning {
			return nil
		}

		if err := e.Stop(workflow); err != nil {
			return err
		}

		time.Sleep(time.Millisecond * 100)
	}
}

// waitForRestart waits for the specified amount of time before restarting
// the DAG.
func waitForRestart(restartWait time.Duration, lg logger.Logger) {
	if restartWait > 0 {
		lg.Info("Waiting for restart", "duration", restartWait)
		time.Sleep(restartWait)
	}
}

func getPreviousExecutionParams(e client.Client, workflow *dag.DAG) (string, error) {
	latestStatus, err := e.GetLatestStatus(workflow)
	if err != nil {
		return "", err
	}

	return latestStatus.Params, nil
}
