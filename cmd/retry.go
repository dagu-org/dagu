package main

import (
	"os"
	"path/filepath"

	"github.com/yohamta/dagu"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"

	"github.com/urfave/cli/v2"
)

func newRetryCommand() *cli.Command {
	return &cli.Command{
		Name:  "retry",
		Usage: "dagu retry --req=<request-id> <DAG file>",
		Flags: append(
			globalFlags,
			&cli.StringFlag{
				Name:     "req",
				Usage:    "request-id",
				Value:    "",
				Required: true,
			},
		),
		Action: func(c *cli.Context) error {
			f, _ := filepath.Abs(c.Args().Get(0))
			db := database.Database{Config: database.DefaultConfig()}
			requestId := c.String("req")
			status, err := db.FindByRequestId(f, requestId)
			if err != nil {
				return err
			}
			d, err := loadDAG(c, c.Args().Get(0), status.Status.Params)
			if err != nil {
				return err
			}
			return retry(d, status)
		},
	}
}

func retry(d *dag.DAG, status *models.StatusFile) error {
	a := &dagu.Agent{
		AgentConfig: &dagu.AgentConfig{
			DAG: d,
			Dry: false,
		},
		RetryConfig: &dagu.RetryConfig{
			Status: status.Status,
		},
	}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}
