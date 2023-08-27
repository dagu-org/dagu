package cmd

import (
	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/models"
	"log"
)

func createStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <DAG file>",
		Short: "Display current status of the DAG",
		Long:  `dagu status <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			loadedDAG, err := loadDAG(args[0], "")
			checkError(err)

			status, err := controller.NewDAGController(loadedDAG).GetStatus()
			checkError(err)

			res := &models.StatusResponse{Status: status}
			log.Printf("Pid=%d Status=%s", res.Status.Pid, res.Status.Status)
		},
	}
}
