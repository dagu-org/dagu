package cmd

import (
	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/persistence/jsondb"
	"github.com/spf13/cobra"
	"path/filepath"
)

func retryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry --req=<request-id> <DAG file>",
		Short: "Retry the DAG execution",
		Long:  `dagu retry --req=<request-id> <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			f, _ := filepath.Abs(args[0])
			reqID, err := cmd.Flags().GetString("req")
			checkError(err)

			status, err := jsondb.New().FindByRequestId(f, reqID)
			checkError(err)

			loadedDAG, err := loadDAG(args[0], status.Status.Params)
			checkError(err)

			a := &agent.Agent{AgentConfig: &agent.AgentConfig{DAG: loadedDAG},
				RetryConfig: &agent.RetryConfig{Status: status.Status}}
			ctx := cmd.Context()
			listenSignals(ctx, a)
			checkError(a.Run(ctx))
		},
	}
	cmd.Flags().StringP("req", "r", "", "request-id")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}
