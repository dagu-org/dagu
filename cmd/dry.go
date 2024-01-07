package cmd

import (
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/client"
	"github.com/spf13/cobra"
)

func dryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] <DAG file>",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		PreRun: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(config.LoadConfig(homeDir))
		},
		Run: func(cmd *cobra.Command, args []string) {
			df := client.NewDataStoreFactory(config.Get())
			e := engine.NewFactory(df, config.Get()).Create()
			execDAG(cmd.Context(), e, cmd, args, true)
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}
