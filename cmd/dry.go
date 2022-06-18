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

var params = ""

// dryCmd represents the dry command
var dryCmd = &cobra.Command{
	Use:   "dry",
	Short: "dagu dry [--params=\"<params>\"] <config>",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl := &config.Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		config_file_path := args[0]
		cfg, err := cl.Load(config_file_path, params)
		if err != nil {
			return err
		}
		return dryRun(cfg)
	},
}

func init() {
	dryCmd.Flags().StringVar(&params, "params", "", "parameters")
	rootCmd.AddCommand(dryCmd)
}

func dryRun(cfg *config.Config) error {
	a := &agent.Agent{AgentConfig: &agent.AgentConfig{
		DAG: cfg,
		Dry: true,
	}}
	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})
	return a.Run()
}
