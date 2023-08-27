/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/dag"
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

const legacyPath = ".dagu"

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dagu/admin.yaml)")

	registerCommands(rootCmd)
}

func initConfig() {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)
	setConfigFile(home)
	cobra.CheckErr(config.LoadConfig(home))
}

func setConfigFile(home string) {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		setDefaultConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName("admin")
	}
}

func setDefaultConfigPath(home string) {
	viper.AddConfigPath(path.Join(home, legacyPath))
}

func loadDAG(dagFile, params string) (d *dag.DAG, err error) {
	dagLoader := &dag.Loader{BaseConfig: config.Get().BaseConfig}
	return dagLoader.Load(dagFile, params)
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
	rootCmd.AddCommand(createStatusCommand())
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(serverCmd())
	rootCmd.AddCommand(createSchedulerCommand())
	rootCmd.AddCommand(retryCmd())
}
