package main

import (
	"os"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu"
	"github.com/yohamta/dagu/internal/config"
)

func newStartCommand() *cli.Command {
	return &cli.Command{
		Name:  "start",
		Usage: "dagu start [--params=\"<params>\"] <config>",
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
			cfg, err := loadDAG(config_file_path, c.String("params"))
			if err != nil {
				return err
			}
			return start(cfg)
		},
	}
}

func start(cfg *config.Config) error {
	a := &dagu.Agent{AgentConfig: &dagu.AgentConfig{
		DAG: cfg,
		Dry: false,
	}}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}
