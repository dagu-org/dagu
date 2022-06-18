package cmd

import (
	"os"
	"path"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/utils"
)

func newServerCommand() *cli.Command {
	l := &admin.Loader{}
	return &cli.Command{
		Name:  "server",
		Usage: "dagu server",
		Action: func(c *cli.Context) error {
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
