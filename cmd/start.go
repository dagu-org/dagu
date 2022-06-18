/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/agent"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"
)

var paramsStart = ""

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "dagu start [--params=\"<params>\"] <config>",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl := &config.Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		config_file_path := args[0]
		cfg, err := cl.Load(config_file_path, paramsStart)
		if err != nil {
			return err
		}
		return start(cfg)
	},
}

func init() {
	startCmd.Flags().StringVar(&params, "params", "", "parameters")
	rootCmd.AddCommand(startCmd)
}

func start(cfg *config.Config) error {
	a := &agent.Agent{AgentConfig: &agent.AgentConfig{
		DAG: cfg,
		Dry: false,
	}}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}
