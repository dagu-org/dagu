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
	rootCmd.AddCommand(cmd.StartCmd())
	rootCmd.AddCommand(cmd.StopCmd())
	rootCmd.AddCommand(cmd.RestartCmd())
	rootCmd.AddCommand(cmd.DryCmd())
	rootCmd.AddCommand(cmd.StatusCmd())
	rootCmd.AddCommand(cmd.VersionCmd())
	rootCmd.AddCommand(cmd.ServerCmd())
	rootCmd.AddCommand(cmd.SchedulerCmd())
	rootCmd.AddCommand(cmd.RetryCmd())
	rootCmd.AddCommand(cmd.StartAllCmd())
}
