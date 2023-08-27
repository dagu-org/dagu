package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/constants"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display the binary version",
		Long:  `dagu version`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(constants.Version)
		},
	}
}
