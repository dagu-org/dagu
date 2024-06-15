package cmd

import (
	"log"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] <DAG file>",
		Short: "Runs the DAG",
		Long:  `dagu start [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig()
			if err != nil {
				log.Fatalf("Failed to load config: %v", err)
			}
			runDAG(cmd.Context(),
				cfg,
				engine.New(
					client.NewDataStoreFactory(cfg),
					engine.DefaultConfig(),
					cfg,
				),
				cmd,
				args,
				false,
			)
		},
	}

	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}
