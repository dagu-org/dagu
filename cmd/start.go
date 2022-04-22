package main

import (
	"errors"
	"jobctl/internal/agent"
	"jobctl/internal/config"
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func newStartCommand() *cli.Command {
	cl := config.NewConfigLoader()
	return &cli.Command{
		Name:  "start",
		Usage: "jobctl start [--params=\"<params>\"] <config>",
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
				return errors.New("config file must be specified.")
			}
			if c.NArg() != 1 {
				return errors.New("too many parameters.")
			}
			config_file_path := c.Args().Get(0)
			cfg, err := cl.Load(config_file_path, c.String("params"))
			if err != nil {
				return err
			}
			return startJob(cfg)
		},
	}
}

func startJob(cfg *config.Config) error {
	a := &agent.Agent{Config: &agent.Config{
		Job: cfg,
		Dry: false,
	}}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	err := a.Run()
	if err != nil {
		log.Printf("running job failed. %v", err)
	}
	return nil
}
