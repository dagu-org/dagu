package cmd

import (
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/spf13/cobra"
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

			// TODO: inject this
			ef := engine.NewFactory()
			e := ef.Create()

			status, err := e.GetStatus(loadedDAG)
			checkError(err)

			res := &model.StatusResponse{Status: status}
			log.Printf("Pid=%d Status=%s", res.Status.Pid, res.Status.Status)
		},
	}
}
