package main

import (
	"log"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
)

func newStopCommand() *cli.Command {
	return &cli.Command{
		Name:  "stop",
		Usage: "dagu stop <DAG file>",
		Flags: globalFlags,
		Action: func(c *cli.Context) error {
			cfg, err := loadDAG(c, c.Args().Get(0), "")
			if err != nil {
				return err
			}
			return stop(cfg)
		},
	}
}

func stop(cfg *dag.DAG) error {
	c := controller.New(cfg)
	log.Printf("Stopping...")
	return c.Stop()
}
