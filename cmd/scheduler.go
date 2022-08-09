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
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "dags",
				Usage:    "DAGs directory",
				Value:    "",
				Required: false,
			},
		},
		Action: func(c *cli.Context) error {
			dagsDir := c.String("dags")
			if dagsDir != "" {
				globalConfig.DAGs = dagsDir
			}
			return startScheduler(globalConfig)
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
