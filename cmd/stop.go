/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/utils"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "dagu stop <config>",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl := &config.Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		config_file_path := args[0]
		cfg, err := cl.Load(config_file_path, "")
		if err != nil {
			return err
		}
		return stop(cfg)
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func stop(cfg *config.Config) error {
	c := controller.New(cfg)
	log.Printf("Stopping...")
	return c.Stop()
}
