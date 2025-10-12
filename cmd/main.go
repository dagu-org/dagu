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
	rootCmd.AddCommand(cli.CmdStart())
	rootCmd.AddCommand(cli.CmdEnqueue())
	rootCmd.AddCommand(cli.CmdDequeue())
	rootCmd.AddCommand(cli.CmdStop())
	rootCmd.AddCommand(cli.CmdRestart())
	rootCmd.AddCommand(cli.CmdDry())
	rootCmd.AddCommand(cli.CmdValidate())
	rootCmd.AddCommand(cli.CmdStatus())
	rootCmd.AddCommand(cli.CmdVersion())
	rootCmd.AddCommand(cli.CmdServer())
	rootCmd.AddCommand(cli.CmdScheduler())
	rootCmd.AddCommand(cli.CmdCoordinator())
	rootCmd.AddCommand(cli.CmdWorker())
	rootCmd.AddCommand(cli.CmdRetry())
	rootCmd.AddCommand(cli.CmdStartAll())
	rootCmd.AddCommand(cli.CmdMigrate())

	config.Version = version
}

var version = "0.0.0"
