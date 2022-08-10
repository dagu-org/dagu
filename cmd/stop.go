package main

import (
	"log"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
)

func newStopCommand() *cli.Command {
	return &cli.Command{
		Name:  "stop",
		Usage: "dagu stop <config>",
		Action: func(c *cli.Context) error {
			config_file_path := c.Args().Get(0)
			cl := &config.Loader{BaseConfig: globalConfig.BaseConfig}
			cfg, err := cl.Load(config_file_path, "")
			if err != nil {
				return err
			}
			return stop(cfg)
		},
	}
}

func stop(cfg *config.Config) error {
	c := controller.New(cfg)
	log.Printf("Stopping...")
	return c.Stop()
}
