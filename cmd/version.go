package main

import (
	"github.com/dagu-org/dagu/internal/build"
	"github.com/spf13/cobra"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display the binary version",
		Long:  `dagu version`,
		Run: func(_ *cobra.Command, _ []string) {
			println(build.Version)
		},
	}
}
