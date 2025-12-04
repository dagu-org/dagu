package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Mcp() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "mcp",
			Short: "Start the MCP Server",
			Long:  "Start the MCP Server",
			Args:  cobra.ArbitraryArgs,
		}, mcpFlags, runMcp,
	)
}

// Command line flags for the mcp command
var mcpFlags = []commandLineFlag{}

// runMcp handles the execution of the mcp command
func runMcp(ctx *Context, args []string) error {
	mcpserver, err := ctx.NewMcpServer()
	if err != nil {
		return fmt.Errorf("failed to initialize mcp server: %w", err)
	}

	if err := mcpserver.Start(ctx); err != nil {
		return fmt.Errorf("failed to start mcp server in directory %s: %w",
			ctx.Config.Paths.DAGsDir, err)
	}

	return nil
}
