package cmd

import (
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
		PreRun: func(cmd *cobra.Command, args []string) {
			cobra.CheckErr(config.LoadConfig(homeDir))
		},
		Run: func(cmd *cobra.Command, args []string) {
			ds := client.NewDataStoreFactory(config.Get())
			e := engine.NewFactory(ds, config.Get()).Create()
			execDAG(cmd.Context(), e, cmd, args, false)
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}
