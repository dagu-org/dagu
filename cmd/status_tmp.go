package cmd

import (
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"

	"github.com/urfave/cli/v2"
)

func newStatusCommand() *cli.Command {
	cl := &config.Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	return &cli.Command{
		Name:  "status",
		Usage: "dagu status <config>",
		Action: func(c *cli.Context) error {
			config_file_path := c.Args().Get(0)
			cfg, err := cl.Load(config_file_path, "")
			if err != nil {
				return err
			}
			return queryStatus(cfg)
		},
	}
}
