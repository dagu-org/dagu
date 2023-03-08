/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/dag"
)

var (
	cfgFile string
	cfg     *admin.Config

	// TODO: Refactor to read environment variables with other admin.Config fields.
	dagsDir string
	port    string
	host    string

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "dagu",
		Short: "YAML-based DAG scheduling tool.",
		Long:  `YAML-based DAG scheduling tool.`,
	}

	version = "0.0.0"
	sigs    chan os.Signal
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
	constants.Version = version
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dagu/admin.yaml)")
	rootCmd.PersistentFlags().StringVar(&dagsDir, "dags", "", "dags location (default is $HOME/.dagu/dags)")
	rootCmd.PersistentFlags().StringVar(&port, "port", "", "admin server port (default is 8080)")
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "admin server host (default is localhost)")

	rootCmd.AddCommand(startCommand())
	rootCmd.AddCommand(stopCommand())
	rootCmd.AddCommand(restartCommand())
	rootCmd.AddCommand(dryCommand())
	rootCmd.AddCommand(statusCommand())
	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(serverCommand())
	rootCmd.AddCommand(schedulerCommand())
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(path.Join(home, legacyPath))
		viper.SetConfigType("yaml")
		viper.SetConfigName("admin")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	var err error
	cfg, err = loadConfig(rootCmd)
	cobra.CheckErr(err)
}

func loadConfig(cmd *cobra.Command) (*admin.Config, error) {
	ldr := &admin.Loader{}
	cfgFileUsed := viper.ConfigFileUsed()

	cfg, err := ldr.LoadAdminConfig(cfgFileUsed)
	if err == admin.ErrConfigNotFound {
		return admin.DefaultConfig()
	} else if err != nil {
		return nil, fmt.Errorf("unable to load config: %w", err)
	}

	// TODO: Use environment variables instead of flags.
	if s, err := cmd.Flags().GetString("dags"); s != "" && err != nil {
		cfg.DAGs = s
	}
	if s, err := cmd.Flags().GetString("port"); s != "" && err != nil {
		cfg.Port = s
	}
	if s, err := cmd.Flags().GetString("host"); s != "" && err != nil {
		cfg.Host = s
	}

	return cfg, nil
}

func loadDAG(dagFile, params string) (d *dag.DAG, err error) {
	dagLoader := &dag.Loader{BaseConfig: cfg.BaseConfig}
	return dagLoader.Load(dagFile, params)
}

func listenSignals(abortFunc func(sig os.Signal)) {
	sigs = make(chan os.Signal, 100)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigs {
			abortFunc(sig)
		}
	}()
}

func getFlagString(cmd *cobra.Command, name, fallback string) string {
	if s, _ := cmd.Flags().GetString(name); s != "" {
		return s
	}
	return fallback
}
