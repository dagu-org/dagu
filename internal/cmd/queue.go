package cmd

import (
	"github.com/spf13/cobra"
)

func CmdQueue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "queue [flags]",
			Short: "Start the queue server for workflow execution",
			Long:  `Launch the queue process.`,
		}, queueFlags, runQueue,
	)
}

var queueFlags = []commandLineFlag{}

func runQueue(_ *Context, _ []string) error {
	panic("unimplemented")
}
