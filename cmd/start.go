package main

import (
	"errors"
	"log"
	"os"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/config"
)

func newStartCommand() *cli.Command {
	cl := config.NewConfigLoader()
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
			if c.NArg() == 0 {
				return errors.New("config file must be specified")
			}
			if c.NArg() != 1 {
				return errors.New("too many parameters")
			}
			config_file_path := c.Args().Get(0)
			cfg, err := cl.Load(config_file_path, c.String("params"))
			if err != nil {
				return err
			}
			return start(cfg)
		},
	}
}

func start(cfg *config.Config) error {
	a := &agent.Agent{Config: &agent.Config{
		DAG: cfg,
		Dry: false,
	}}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	err := a.Run()
	if err != nil {
		log.Printf("running failed. %v", err)
	}
	return nil
}
