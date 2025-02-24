package cmd

import (
	"github.com/dagu-org/dagu/internal/build"
	"github.com/spf13/cobra"
)

func CmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display the binary version",
		Long:  `Print the current version and build details of the Dagu executable.`,
		Run: func(_ *cobra.Command, _ []string) {
			println(build.Version)
		},
	}
}
