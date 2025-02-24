package main

import (
	"os"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/spf13/cobra"
)

var (
	// version is set at build time
	version = "0.0.0"
	rootCmd = &cobra.Command{
		Use:   build.Slug,
		Short: "YAML-based DAG scheduling tool.",
		Long:  `YAML-based DAG scheduling tool.`,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	build.Version = version

	registerCommands()
}

func registerCommands() {
	rootCmd.AddCommand(cmd.CmdStart())
	rootCmd.AddCommand(cmd.CmdStop())
	rootCmd.AddCommand(cmd.CmdRestart())
	rootCmd.AddCommand(cmd.CmdDry())
	rootCmd.AddCommand(cmd.CmdStatus())
	rootCmd.AddCommand(cmd.CmdVersion())
	rootCmd.AddCommand(cmd.CmdServer())
	rootCmd.AddCommand(cmd.CmdScheduler())
	rootCmd.AddCommand(cmd.CmdRetry())
	rootCmd.AddCommand(cmd.CmdStartAll())
}
