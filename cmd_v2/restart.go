package cmd_v2

import (
	"log"
	"time"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
)

func restartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <DAG file>",
		Short: "Restart specified DAG",
		Long:  `dagu restart <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dagFile := args[0]
			d, err := loadDAG(dagFile, "")
			cobra.CheckErr(err)

			ctrl := controller.NewDAGController(d)

			// Check the current status.
			st, err := ctrl.GetStatus()
			cobra.CheckErr(err)

			// Stop the DAG if it is running.
			if st.Status == scheduler.SchedulerStatus_Running {
				log.Printf("Stopping %s for restart...", d.Name)
				cobra.CheckErr(stopRunningDAG(ctrl))
			}

			// Wait for the specified amount of time before restarting.
			if d.RestartWait > 0 {
				log.Printf("Waiting for %s...", d.RestartWait)
				time.Sleep(d.RestartWait)
			}

			// Retrieve the parameter of the previous execution.
			log.Printf("Restarting %s...", d.Name)
			st, err = ctrl.GetLastStatus()
			cobra.CheckErr(err)

			// Start the DAG with the same parmaeter.
			d, err = loadDAG(dagFile, st.Params)
			cobra.CheckErr(err)
			cobra.CheckErr(start(d, false))
		},
	}
}

func stopRunningDAG(ctrl *controller.DAGController) error {
	for {
		st, err := ctrl.GetStatus()
		cobra.CheckErr(err)

		if st.Status != scheduler.SchedulerStatus_Running {
			return nil
		}
		cobra.CheckErr(ctrl.Stop())
		time.Sleep(time.Millisecond * 100)
	}
}
