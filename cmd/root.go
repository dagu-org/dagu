package cmd

import (
	"github.com/daguflow/dagu/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// cfgFile parameter
	cfgFile string

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "dagu",
		Short: "YAML-based DAG scheduling tool.",
		Long:  `YAML-based DAG scheduling tool.`,
	}
)

// Execute adds all child commands to the root command and sets flags
// appropriately. This is called by main.main(). It only needs to happen
// once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
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

func init() {
	rootCmd.PersistentFlags().
		StringVar(
			&cfgFile, "config", "",
			"config file (default is $HOME/.config/dagu/admin.yaml)",
		)

	cobra.OnInitialize(initialize)

	registerCommands()
}

func initialize() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		return
	}

	viper.AddConfigPath(config.ConfigDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")
}
