package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

func dryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] <DAG file>",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			params, err := cmd.Flags().GetString("params")
			cobra.CheckErr(err)
			d, err := loadDAG(args[0], strings.Trim(params, `"`))
			cobra.CheckErr(err)
			cobra.CheckErr(start(d, true))
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}
