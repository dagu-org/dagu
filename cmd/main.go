package main

import (
	"os"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/spf13/cobra"

	_ "github.com/dagu-org/dagu/internal/runtime/builtin" // Register built-in executors
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
	rootCmd.AddCommand(cmd.Start())
	rootCmd.AddCommand(cmd.Exec())
	rootCmd.AddCommand(cmd.Enqueue())
	rootCmd.AddCommand(cmd.Dequeue())
	rootCmd.AddCommand(cmd.Stop())
	rootCmd.AddCommand(cmd.Restart())
	rootCmd.AddCommand(cmd.Dry())
	rootCmd.AddCommand(cmd.Validate())
	rootCmd.AddCommand(cmd.Status())
	rootCmd.AddCommand(cmd.History())
	rootCmd.AddCommand(cmd.Version())
	rootCmd.AddCommand(cmd.Server())
	rootCmd.AddCommand(cmd.Scheduler())
	rootCmd.AddCommand(cmd.CmdCoordinator())
	rootCmd.AddCommand(cmd.CmdWorker())
	rootCmd.AddCommand(cmd.Retry())
	rootCmd.AddCommand(cmd.StartAll())
	rootCmd.AddCommand(cmd.Migrate())
	rootCmd.AddCommand(cmd.Cleanup())
	rootCmd.AddCommand(cmd.Sync())
	rootCmd.AddCommand(cmd.Upgrade())
	rootCmd.AddCommand(cmd.License())

	config.Version = version
}

var version = "0.0.0"
