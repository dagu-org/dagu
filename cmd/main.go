package main

import (
	"os"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   build.Slug,
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
	rootCmd.AddCommand(cmd.CmdStart())
	rootCmd.AddCommand(cmd.CmdEnqueue())
	rootCmd.AddCommand(cmd.CmdDequeue())
	rootCmd.AddCommand(cmd.CmdStop())
	rootCmd.AddCommand(cmd.CmdRestart())
	rootCmd.AddCommand(cmd.CmdDry())
	rootCmd.AddCommand(cmd.CmdValidate())
	rootCmd.AddCommand(cmd.CmdStatus())
	rootCmd.AddCommand(cmd.CmdVersion())
	rootCmd.AddCommand(cmd.CmdServer())
	rootCmd.AddCommand(cmd.CmdScheduler())
	rootCmd.AddCommand(cmd.CmdCoordinator())
	rootCmd.AddCommand(cmd.CmdWorker())
	rootCmd.AddCommand(cmd.CmdRetry())
	rootCmd.AddCommand(cmd.CmdStartAll())
	rootCmd.AddCommand(cmd.CmdMigrate())

	build.Version = version
}

var version = "0.0.0"
