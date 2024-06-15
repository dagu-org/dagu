package cmd

import (
	"log"
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
			cobra.CheckErr(config.LoadConfig())
		},
		Run: func(cmd *cobra.Command, args []string) {
			reqID, err := cmd.Flags().GetString("req")
			if err != nil {
				log.Fatalf("Request ID is required: %v", err)
			}

			// Read the specified DAG execution status from the history store.
			dataStore := client.NewDataStoreFactory(config.Get())
			historyStore := dataStore.NewHistoryStore()

			absoluteFilePath, err := filepath.Abs(args[0])
			if err != nil {
				log.Fatalf("Failed to get the absolute path of the DAG file: %v", err)
			}

			status, err := historyStore.FindByRequestId(absoluteFilePath, reqID)
			if err != nil {
				log.Fatalf("Failed to find the request: %v", err)
			}

			// Start the DAG with the same parameters with the execution that is being retried.
			loadedDAG, err := loadDAG(args[0], status.Status.Params)
			if err != nil {
				log.Fatalf("Failed to load DAG: %v", err)
			}

			dagAgent := agent.New(
				&agent.Config{DAG: loadedDAG, RetryTarget: status.Status},
				engine.New(dataStore, engine.DefaultConfig(), config.Get()),
				dataStore,
			)

			ctx := cmd.Context()
			listenSignals(ctx, dagAgent)

			if err := dagAgent.Run(ctx); err != nil {
				log.Fatalf("Failed to start the DAG: %v", err)
			}
		},
	}

	cmd.Flags().StringP("req", "r", "", "request-id")
	_ = cmd.MarkFlagRequired("req")
	return cmd
}
