package cmd

import (
	"log"
	"time"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/spf13/cobra"
)

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <DAG file>",
		Short: "Restart the DAG",
		Long:  `dagu restart <DAG file>`,
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(config.LoadConfig())
		},
		Run: func(cmd *cobra.Command, args []string) {
			// Load the DAG file and stop the DAG if it is running.
			dagFilePath := args[0]
			dg, err := loadDAG(dagFilePath, "")
			if err != nil {
				log.Fatalf("Failed to load DAG: %v", err)
			}

			eng := engine.New(
				client.NewDataStoreFactory(config.Get()),
				engine.DefaultConfig(),
				config.Get(),
			)

			if err := stopDAGIfRunning(eng, dg); err != nil {
				log.Fatalf("Failed to stop the DAG: %v", err)
			}

			// Wait for the specified amount of time before restarting.
			waitForRestart(dg.RestartWait)

			// Retrieve the parameter of the previous execution.
			log.Printf("Restarting %s...", dg.Name)
			params, err := getPreviousExecutionParams(eng, dg)
			if err != nil {
				log.Fatalf("Failed to get previous execution params: %v", err)
			}

			// Start the DAG with the same parameter.
			// Need to reload the DAG file with the parameter.
			dg, err = loadDAG(dagFilePath, params)
			if err != nil {
				log.Fatalf("Failed to load DAG: %v", err)
			}

			cobra.CheckErr(start(cmd.Context(), eng, dg, false))
		},
	}
}

// stopDAGIfRunning stops the DAG if it is running.
// Otherwise, it does nothing.
func stopDAGIfRunning(e engine.Engine, dg *dag.DAG) error {
	st, err := e.GetCurrentStatus(dg)
	if err != nil {
		return err
	}

	if st.Status == scheduler.StatusRunning {
		log.Printf("Stopping %s for restart...", dg.Name)
		cobra.CheckErr(stopRunningDAG(e, dg))
	}
	return nil
}

// stopRunningDAG attempts to stop the running DAG
// by sending a stop signal to the agent.
func stopRunningDAG(e engine.Engine, dg *dag.DAG) error {
	for {
		curStatus, err := e.GetCurrentStatus(dg)
		if err != nil {
			return err
		}

		// If the DAG is not running, do nothing.
		if curStatus.Status != scheduler.StatusRunning {
			return nil
		}

		if err := e.Stop(dg); err != nil {
			return err
		}

		time.Sleep(time.Millisecond * 100)
	}
}

// waitForRestart waits for the specified amount of time before restarting the DAG.
func waitForRestart(restartWait time.Duration) {
	if restartWait > 0 {
		log.Printf("Waiting for %s...", restartWait)
		time.Sleep(restartWait)
	}
}

func getPreviousExecutionParams(e engine.Engine, dg *dag.DAG) (string, error) {
	latestStatus, err := e.GetLatestStatus(dg)
	if err != nil {
		return "", err
	}

	return latestStatus.Params, nil
}
