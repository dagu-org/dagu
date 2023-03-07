package cmd_v2

import (
	"fmt"
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
			cobra.CheckErr(restart(args[0]))
		},
	}
}

func restart(dagFile string) error {
	d, err := loadDAG(dagFile, "")
	if err != nil {
		return fmt.Errorf("error loading DAG file: %w", err)
	}
	ctrl := controller.NewDAGController(d)

	// Check the current status.
	st, err := ctrl.GetStatus()
	if err != nil {
		return fmt.Errorf("error reading current status: %v", err)
	}

	// Stop the DAG if it is running.
	if st.Status == scheduler.SchedulerStatus_Running {
		log.Printf("Stopping %s for restart...", d.Name)
		if err := stopRunningDAG(ctrl); err != nil {
			return err
		}
	}

	// Wait for the specified amount of time before restarting.
	if d.RestartWait > 0 {
		log.Printf("Waiting for %s...", d.RestartWait)
		time.Sleep(d.RestartWait)
	}

	// Retrieve the parameter of the previous execution.
	log.Printf("Restarting %s...", d.Name)
	st, err = ctrl.GetLastStatus()
	if err != nil {
		return fmt.Errorf("error reading the last status: %w", err)
	}

	// Start the DAG with the same parmaeter.
	d, err = loadDAG(dagFile, st.Params)
	if err != nil {
		return err
	}
	return start(d, false)
}

func stopRunningDAG(ctrl *controller.DAGController) error {
	for {
		st, err := ctrl.GetStatus()
		if err != nil {
			return fmt.Errorf("error reading DAG status: %w", err)
		}
		if st.Status != scheduler.SchedulerStatus_Running {
			return nil
		}
		if err := ctrl.Stop(); err != nil {
			return fmt.Errorf("error stopping the DAG: %w", err)
		}
		time.Sleep(time.Millisecond * 500)
	}
}
