package main

import (
	"os"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/runner"
)

func newSchedulerCommand() *cli.Command {
	return &cli.Command{
		Name:  "scheduler",
		Usage: "dagu scheduler",
		Flags: append(
			globalFlags,
			&cli.StringFlag{
				Name:     "dags",
				Usage:    "DAGs directory",
				Value:    "",
				Required: false,
			},
		),
		Action: func(c *cli.Context) error {
			cfg, err := loadGlobalConfig(c)
			if err != nil {
				return err
			}
			return startScheduler(cfg)
		},
	}
}

func startScheduler(cfg *admin.Config) error {
	agent := runner.NewAgent(cfg)

	listenSignals(func(sig os.Signal) {
		agent.Stop()
	})

	return agent.Start()
}
