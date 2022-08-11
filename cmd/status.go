package main

import (
	"log"

	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/models"

	"github.com/urfave/cli/v2"
)

func newStatusCommand() *cli.Command {
	return &cli.Command{
		Name:  "status",
		Usage: "dagu status <DAG file>",
		Flags: globalFlags,
		Action: func(c *cli.Context) error {
			d, err := loadDAG(c, c.Args().Get(0), "")
			if err != nil {
				return err
			}
			return queryStatus(d)
		},
	}
}

func queryStatus(d *dag.DAG) error {
	status, err := controller.New(d).GetStatus()
	if err != nil {
		return err
	}
	res := &models.StatusResponse{
		Status: status,
	}
	log.Printf("Pid=%d Status=%s", res.Status.Pid, res.Status.Status)
	return nil
}
