package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/database"
)

func retryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry --req=<request-id> <DAG file>",
		Short: "Retry the DAG execution",
		Long:  `dagu retry --req=<request-id> <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			f, _ := filepath.Abs(args[0])
			reqID, err := cmd.Flags().GetString("req")
			cobra.CheckErr(err)
			status, err := database.New().FindByRequestId(f, reqID)
			cobra.CheckErr(err)
			d, err := loadDAG(args[0], status.Status.Params)
			cobra.CheckErr(err)
			a := &agent.Agent{AgentConfig: &agent.AgentConfig{DAG: d},
				RetryConfig: &agent.RetryConfig{Status: status.Status}}
			listenSignals(func(sig os.Signal) { a.Signal(sig) })
			cobra.CheckErr(a.Run())
		},
	}
	cmd.Flags().StringP("req", "r", "", "request-id")
	cmd.MarkFlagRequired("req")
	return cmd
}
