/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"path"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "dagu",
		Short: "YAML-based DAG scheduling tool.",
		Long:  `YAML-based DAG scheduling tool.`,
	}
)

const configPath = ".dagu"

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dagu/admin.yaml)")

	cobra.OnInitialize(initialize)

	registerCommands(rootCmd)
}

var (
	homeDir string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		cobra.CheckErr(err)
	}
	homeDir = home
}

func initialize() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		setDefaultConfigPath()
		viper.SetConfigType("yaml")
		viper.SetConfigName("admin")
	}
}

func setDefaultConfigPath() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic("could not determine home directory")
	}
	viper.AddConfigPath(path.Join(homeDir, configPath))
}

func loadDAG(dagFile, params string) (d *dag.DAG, err error) {
	return dag.Load(config.Get().BaseConfig, dagFile, params)
}

func getFlagString(cmd *cobra.Command, name, fallback string) string {
	if s, _ := cmd.Flags().GetString(name); s != "" {
		return s
	}
	return fallback
}

func registerCommands(root *cobra.Command) {
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
