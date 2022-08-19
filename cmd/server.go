package main

import (
	"os"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/utils"
)

func newServerCommand() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "dagu server",
		Flags: append(globalFlags,
			&cli.StringFlag{
				Name:     "dags",
				Usage:    "DAGs directory",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "port",
				Usage:    "server port",
				Value:    "",
				Required: false,
			},
			&cli.StringFlag{
				Name:     "host",
				Usage:    "server host",
				Value:    "",
				Required: false,
			},
		),
		Action: func(c *cli.Context) error {
			cfg, err := loadGlobalConfig(c)
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

	err := server.Serve()
	utils.LogErr("running server", err)
	return err
}
