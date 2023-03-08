package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/models"
)

func statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <DAG file>",
		Short: "Display current status of the DAG",
		Long:  `dagu status <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			d, err := loadDAG(args[0], "")
			cobra.CheckErr(err)

			status, err := controller.NewDAGController(d).GetStatus()
			cobra.CheckErr(err)

			res := &models.StatusResponse{Status: status}
			log.Printf("Pid=%d Status=%s", res.Status.Pid, res.Status.Status)
		},
	}
}
