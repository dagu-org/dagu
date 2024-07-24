// Copyright (c) 2022-2024 Daguflow Inc.

package main

import (
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile parameter
	cfgFile string

	// version of the application
	// This variable is set at build time using ldflags
	version = "0.0.0"
)

func main() {
	// Set the version to the constants package
	constants.Version = version

	// Initialize cobra
	cobra.OnInitialize(onInitialize)

	// Set up the root command
	cmd := &cobra.Command{
		Use:   "dagu",
		Short: "YAML-based DAG scheduling tool.",
		Long:  `YAML-based DAG scheduling tool.`,
	}

	cmd.PersistentFlags().
		StringVar(
			&cfgFile, "config", "",
			"config file (default is $HOME/.config/dagu/admin.yaml)",
		)

	cmd.AddCommand(startCmd())
	cmd.AddCommand(stopCmd())
	cmd.AddCommand(restartCmd())
	cmd.AddCommand(dryCmd())
	cmd.AddCommand(statusCmd())
	cmd.AddCommand(versionCmd())
	cmd.AddCommand(serverCmd())
	cmd.AddCommand(schedulerCmd())
	cmd.AddCommand(retryCmd())
	cmd.AddCommand(startAllCmd())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func onInitialize() {
	// Viper configuration
	viper.AddConfigPath(config.ConfigDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}
}
