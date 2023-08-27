package cmd

import "github.com/spf13/cobra"

func dryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] <DAG file>",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			execDAG(cmd.Context(), cmd, args, true)
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}
