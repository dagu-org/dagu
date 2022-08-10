package main

import (
	"os"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu"
	"github.com/yohamta/dagu/internal/config"
)

func newDryCommand() *cli.Command {
	return &cli.Command{
		Name:  "dry",
		Usage: "dagu dry [--params=\"<params>\"] <config>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "params",
				Usage:    "parameters",
				Value:    "",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			config_file_path := c.Args().Get(0)
			cl := &config.Loader{BaseConfig: globalConfig.BaseConfig}
			cfg, err := cl.Load(config_file_path, c.String("params"))
			if err != nil {
				return err
			}
			return dryRun(cfg)
		},
	}
}

func dryRun(cfg *config.Config) error {
	a := &dagu.Agent{AgentConfig: &dagu.AgentConfig{
		DAG: cfg,
		Dry: true,
	}}
	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})
	return a.Run()
}
