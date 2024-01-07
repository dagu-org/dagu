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
			cobra.CheckErr(config.LoadConfig(homeDir))
		},
		Run: func(cmd *cobra.Command, args []string) {
			dagFile := args[0]
			loadedDAG, err := loadDAG(dagFile, "")
			checkError(err)

			df := client.NewDataStoreFactory(config.Get())
			e := engine.NewFactory(df, config.Get()).Create()

			// Check the current status and stop the DAG if it is running.
			stopDAGIfRunning(e, loadedDAG)

			// Wait for the specified amount of time before restarting.
			waitForRestart(loadedDAG.RestartWait)

			// Retrieve the parameter of the previous execution.
			log.Printf("Restarting %s...", loadedDAG.Name)
			params := getPreviousExecutionParams(e, loadedDAG)

			// Start the DAG with the same parameter.
			loadedDAG, err = loadDAG(dagFile, params)
			checkError(err)
			cobra.CheckErr(start(cmd.Context(), e, loadedDAG, false))
		},
	}
}

func stopDAGIfRunning(e engine.Engine, dag *dag.DAG) {
	st, err := e.GetCurrentStatus(dag)
	checkError(err)

	// Stop the DAG if it is running.
	if st.Status == scheduler.SchedulerStatus_Running {
		log.Printf("Stopping %s for restart...", dag.Name)
		cobra.CheckErr(stopRunningDAG(e, dag))
	}
}

func stopRunningDAG(e engine.Engine, dag *dag.DAG) error {
	for {
		st, err := e.GetCurrentStatus(dag)
		checkError(err)

		if st.Status != scheduler.SchedulerStatus_Running {
			return nil
		}
		checkError(e.Stop(dag))
		time.Sleep(time.Millisecond * 100)
	}
}

func waitForRestart(restartWait time.Duration) {
	if restartWait > 0 {
		log.Printf("Waiting for %s...", restartWait)
		time.Sleep(restartWait)
	}
}

func getPreviousExecutionParams(e engine.Engine, dag *dag.DAG) string {
	st, err := e.GetLatestStatus(dag)
	checkError(err)

	return st.Params
}
