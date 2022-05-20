package main

import (
	"os"
	"path/filepath"

	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/utils"

	"github.com/urfave/cli/v2"
)

func newRetryCommand() *cli.Command {
	return &cli.Command{
		Name:  "retry",
		Usage: "dagu retry --req=<request-id> <config>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "req",
				Usage:    "request-id",
				Value:    "",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			f, _ := filepath.Abs(c.Args().Get(0))
			requestId := c.String("req")
			return retry(f, requestId)
		},
	}
}

func retry(f, requestId string) error {
	cl := &config.Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	db := database.New(database.DefaultConfig())
	status, err := db.FindByRequestId(f, requestId)
	if err != nil {
		return err
	}

	cfg, err := cl.Load(f, status.Status.Params)
	if err != nil {
		return err
	}

	a := &agent.Agent{
		Config: &agent.Config{
			DAG: cfg,
			Dry: false,
		},
		RetryConfig: &agent.RetryConfig{
			Status: status.Status,
		},
	}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}
