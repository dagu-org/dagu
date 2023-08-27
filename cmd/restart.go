package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"github.com/yohamta/dagu/internal/scheduler"
	"log"
	"time"
)

func restartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <DAG file>",
		Short: "Restart the DAG",
		Long:  `dagu restart <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dagFile := args[0]
			loadedDAG, err := loadDAG(dagFile, "")
			checkError(err)

			ctrl := controller.New(loadedDAG, jsondb.New())

			// Check the current status and stop the DAG if it is running.
			stopDAGIfRunning(ctrl)

			// Wait for the specified amount of time before restarting.
			waitForRestart(loadedDAG.RestartWait)

			// Retrieve the parameter of the previous execution.
			log.Printf("Restarting %s...", loadedDAG.Name)
			params := getPreviousExecutionParams(ctrl)

			// Start the DAG with the same parameter.
			loadedDAG, err = loadDAG(dagFile, params)
			checkError(err)
			cobra.CheckErr(start(cmd.Context(), loadedDAG, false))
		},
	}
}

func stopDAGIfRunning(ctrl *controller.DAGController) {
	st, err := ctrl.GetStatus()
	checkError(err)

	// Stop the DAG if it is running.
	if st.Status == scheduler.SchedulerStatus_Running {
		log.Printf("Stopping %s for restart...", ctrl.DAG.Name)
		cobra.CheckErr(stopRunningDAG(ctrl))
	}
}

func stopRunningDAG(ctrl *controller.DAGController) error {
	for {
		st, err := ctrl.GetStatus()
		checkError(err)

		if st.Status != scheduler.SchedulerStatus_Running {
			return nil
		}
		checkError(ctrl.Stop())
		time.Sleep(time.Millisecond * 100)
	}
}

func waitForRestart(restartWait time.Duration) {
	if restartWait > 0 {
		log.Printf("Waiting for %s...", restartWait)
		time.Sleep(restartWait)
	}
}

func getPreviousExecutionParams(ctrl *controller.DAGController) string {
	st, err := ctrl.GetLastStatus()
	checkError(err)

	return st.Params
}
