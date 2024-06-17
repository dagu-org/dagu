package cmd

import (
	"fmt"

	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/spf13/cobra"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display the binary version",
		Long:  `dagu version`,
		Run: func(_ *cobra.Command, args []string) {
			// nolint // forbidgo
			fmt.Println(constants.Version)
		},
	}
}
