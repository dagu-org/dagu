package cmd

import "github.com/spf13/cobra"

func Mcp() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "mcp",
			Short: "Start the MCP Server",
			Long:  "Satrt the MCP Server",
			Args:  cobra.ArbitraryArgs,
		}, mcpFlags, runMcp,
	)
}

// Command line flags for the mcp command
var mcpFlags = []commandLineFlag{}

// runMcp handles the execution of the mcp command
func runMcp(ctx *Context, args []string) error {
	return nil
}
