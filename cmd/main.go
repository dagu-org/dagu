// Copyright (c) 2022-2024 Daguflow Inc.

package main

import (
	"log"
	"os"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/constants"
	"github.com/dagu-dev/dagu/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile parameter
	cfgFile string

	// quiet parameter
	quiet bool

	// appConfig is the configuration object
	appConfig *config.Config

	// version of the application
	// This variable is set at build time using ldflags
	version = "0.0.0"

	// appLogger is the logger object
	appLogger logger.Logger
)

func main() {
	// Set the version to the constants package
	constants.Version = version

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

	cmd.PersistentFlags().BoolVarP(
		&quiet, "quiet", "q", false, "Run in quiet mode",
	)

	cmd.AddCommand(startCommand())
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

func initialize(cmd *cobra.Command) {
	if err := cmd.ParseFlags(os.Args); err != nil {
		log.Fatalf("Command parse failed: %v", err)
	}

	// Load the configuration
	viper.AddConfigPath(config.ConfigDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Configuration load failed: %v", err)
	}
	appConfig = cfg

	appLogger = logger.NewLogger(logger.NewLoggerArgs{
		LogLevel:  appConfig.LogLevel,
		LogFormat: appConfig.LogFormat,
		Quiet:     quiet,
	})
}
