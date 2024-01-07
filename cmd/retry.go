package cmd

import (
	"path/filepath"

	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/spf13/cobra"
)

func retryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry --req=<request-id> <DAG file>",
		Short: "Retry the DAG execution",
		Long:  `dagu retry --req=<request-id> <DAG file>`,
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(config.LoadConfig(homeDir))
		},
		Run: func(cmd *cobra.Command, args []string) {
			f, _ := filepath.Abs(args[0])
			reqID, err := cmd.Flags().GetString("req")
			checkError(err)

			// TODO: use engine.Engine instead of client.DataStoreFactory
			df := client.NewDataStoreFactory(config.Get())
			e := engine.NewFactory(df, nil).Create()

			hs := df.NewHistoryStore()

			status, err := hs.FindByRequestId(f, reqID)
			checkError(err)

			loadedDAG, err := loadDAG(args[0], status.Status.Params)
			checkError(err)

			a := agent.New(&agent.Config{DAG: loadedDAG, RetryTarget: status.Status}, e, df)
			ctx := cmd.Context()
			listenSignals(ctx, a)
			checkError(a.Run(ctx))
		},
	}
	cmd.Flags().StringP("req", "r", "", "request-id")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}
