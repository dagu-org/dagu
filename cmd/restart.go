package cmd

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/client"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/dag/scheduler"
	"github.com/dagu-dev/dagu/internal/logger"
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
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
				Quiet:     quiet,
			})

			// Load the DAG file and stop the DAG if it is running.
			specFilePath := args[0]
			workflow, err := dag.Load(cfg.BaseConfig, specFilePath, "")
			if err != nil {
				initLogger.Error("Workflow load failed", "error", err, "file", args[0])
				os.Exit(1)
			}

			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, initLogger)

			if err := stopDAGIfRunning(cli, workflow, initLogger); err != nil {
				initLogger.Error("Workflow stop operation failed",
					"error", err,
					"workflow", workflow.Name)
				os.Exit(1)
			}

			// Wait for the specified amount of time before restarting.
			waitForRestart(workflow.RestartWait, initLogger)

			// Retrieve the parameter of the previous execution.
			params, err := getPreviousExecutionParams(cli, workflow)
			if err != nil {
				initLogger.Error("Previous execution parameter retrieval failed",
					"error", err,
					"workflow", workflow.Name)
				os.Exit(1)
			}

			// Start the DAG with the same parameter.
			// Need to reload the DAG file with the parameter.
			workflow, err = dag.Load(cfg.BaseConfig, specFilePath, params)
			if err != nil {
				initLogger.Error("Workflow reload failed",
					"error", err,
					"file", specFilePath,
					"params", params)
				os.Exit(1)
			}

			requestID, err := generateRequestID()
			if err != nil {
				initLogger.Error("Request ID generation failed", "error", err)
				os.Exit(1)
			}

			logFile, err := openLogFile("restart_", cfg.LogDir, workflow, requestID)
			if err != nil {
				initLogger.Error("Log file creation failed",
					"error", err,
					"workflow", workflow.Name)
				os.Exit(1)
			}
			defer logFile.Close()

			agentLogger := logger.NewLogger(logger.NewLoggerArgs{
				LogLevel:  cfg.LogLevel,
				LogFormat: cfg.LogFormat,
				LogFile:   logFile,
				Quiet:     quiet,
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
				newClient(cfg, dataStore, agentLogger),
				dataStore,
				&agent.Options{Dry: false})

			listenSignals(cmd.Context(), agt)
			if err := agt.Run(cmd.Context()); err != nil {
				agentLogger.Error("Workflow restart failed",
					"error", err,
					"workflow", workflow.Name,
					"requestID", requestID)
				os.Exit(1)
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
