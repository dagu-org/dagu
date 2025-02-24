package main

import (
	"os"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   build.Slug,
	Short: "A compact, portable, and language-agnostic workflow engine.",
	Long:  `Dagu is a comprehensive workflow orchestration tool that leverages a simple, human-readable YAML syntax to define and execute directed acyclic graphs (DAGs) of tasks. It supports advanced features such as parameterized execution (both positional and named), dry-run simulations, automated retries, and real-time status monitoring. With built-in integration for multiple executors (Docker, HTTP, SSH, mail, etc.) and an intuitive web UI, Dagu streamlines the management of job dependencies, error recovery, and logging across both local and production environments. Its modular architecture and minimal configuration requirements make it an ideal solution for automating complex processes while ensuring flexibility and reliability.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
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

	build.Version = version
}

var version = "0.0.0"
