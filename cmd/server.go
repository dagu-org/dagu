/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/utils"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "dagu server",
	RunE: func(cmd *cobra.Command, args []string) error {
		l := &admin.Loader{}
		cfg, err := l.LoadAdminConfig(
			path.Join(utils.MustGetUserHomeDir(), ".dagu/admin.yaml"))
		if err == admin.ErrConfigNotFound {
			cfg = admin.DefaultConfig()
		} else if err != nil {
			return err
		}
		return startServer(cfg)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}

func startServer(cfg *admin.Config) error {
	server := admin.NewServer(cfg)

	listenSignals(func(sig os.Signal) {
		server.Shutdown()
	})

	err := server.Serve()
	utils.LogErr("running server", err)
	return err
}
