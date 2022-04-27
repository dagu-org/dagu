package main

import (
	"os"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/admin"
)

func newServerCommand() *cli.Command {
	cl := admin.NewConfigLoader()
	return &cli.Command{
		Name:  "server",
		Usage: "dagu server",
		Action: func(c *cli.Context) error {
			cfg, err := cl.LoadAdminConfig("")
			if err == admin.ErrConfigNotFound {
				cfg, err = admin.DefaultConfig()
			}
			if err != nil {
				return err
			}
			return startServer(cfg)
		},
	}
}

func startServer(cfg *admin.Config) error {
	server := admin.NewServer(cfg)

	listenSignals(func(sig os.Signal) {
		server.Shutdown()
	})

	return server.Serve()
}
