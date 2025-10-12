package main

import (
	"os"

	"github.com/dagu-org/dagu/internal/cli"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/spf13/cobra"

	_ "github.com/dagu-org/dagu/internal/digraph/executor" // Register built-in executors
)

var rootCmd = &cobra.Command{
	Use:   config.AppSlug,
	Short: "Dagu is a compact, portable workflow engine",
	Long: `Dagu is a compact, portable workflow engine.

It provides a declarative model for orchestrating command execution across
diverse environments, including shell scripts, Python commands, containerized
operations, or remote commands.
`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(cli.Start())
	rootCmd.AddCommand(cli.Enqueue())
	rootCmd.AddCommand(cli.Dequeue())
	rootCmd.AddCommand(cli.Stop())
	rootCmd.AddCommand(cli.Restart())
	rootCmd.AddCommand(cli.Dry())
	rootCmd.AddCommand(cli.Validate())
	rootCmd.AddCommand(cli.Status())
	rootCmd.AddCommand(cli.Version())
	rootCmd.AddCommand(cli.Server())
	rootCmd.AddCommand(cli.Scheduler())
	rootCmd.AddCommand(cli.CmdCoordinator())
	rootCmd.AddCommand(cli.CmdWorker())
	rootCmd.AddCommand(cli.Retry())
	rootCmd.AddCommand(cli.StartAll())
	rootCmd.AddCommand(cli.Migrate())

	config.Version = version
}

var version = "0.0.0"
