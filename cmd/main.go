package main

import (
	"fmt"
	"os"

	"github.com/dagu-org/dagu/internal/build"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(restartCmd())
	rootCmd.AddCommand(dryCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(serverCmd())
	rootCmd.AddCommand(schedulerCmd())
	rootCmd.AddCommand(retryCmd())
	rootCmd.AddCommand(startAllCmd())
}

type commandLineFlag struct {
	name, shorthand, defaultValue, usage string
	required                             bool
}

var configFlag = commandLineFlag{
	name:      "config",
	shorthand: "c",
	usage:     "config file (default is $HOME/.config/dagu/config.yaml)",
}

// initCommonFlags initializes common flags for the command
func initCommonFlags(cmd *cobra.Command, addFlags []commandLineFlag) {
	addFlags = append(addFlags, configFlag)
	for _, flag := range addFlags {
		cmd.Flags().StringP(flag.name, flag.shorthand, flag.defaultValue, flag.usage)
		if flag.required {
			if err := cmd.MarkFlagRequired(flag.name); err != nil {
				fmt.Printf("failed to mark flag %s as required: %v\n", flag.name, err)
			}
		}
	}
}

// bindCommonFlags binds common flags to the command
func bindCommonFlags(cmd *cobra.Command, addFlags []string) error {
	flags := []string{"config"}
	flags = append(flags, addFlags...)
	for _, flag := range flags {
		if err := viper.BindPFlag(flag, cmd.Flags().Lookup(flag)); err != nil {
			return fmt.Errorf("failed to bind flag %s: %w", flag, err)
		}
	}
	return nil
}
