/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/utils"
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "dagu status <config>",
	RunE: func(cmd *cobra.Command, args []string) error {
		cl := &config.Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		config_file_path := args[0]
		cfg, err := cl.Load(config_file_path, "")
		if err != nil {
			return err
		}
		return queryStatus(cfg)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func queryStatus(cfg *config.Config) error {
	status, err := controller.New(cfg).GetStatus()
	if err != nil {
		return err
	}
	res := &models.StatusResponse{
		Status: status,
	}
	log.Printf("Pid=%d Status=%s", res.Status.Pid, res.Status.Status)
	return nil
}
