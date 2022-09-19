package main

import (
	"fmt"
	"log"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/scheduler"
)

func newRestartCommand() *cli.Command {
	return &cli.Command{
		Name:  "restart",
		Usage: "dagu restart <DAG file>",
		Flags: globalFlags,
		Action: func(c *cli.Context) error {
			d, err := loadDAG(c, c.Args().Get(0), "")
			if err != nil {
				return err
			}
			return restart(d, c)
		},
	}
}

const resetartTimeout = time.Second * 180

func restart(d *dag.DAG, ctx *cli.Context) error {
	c := controller.NewDAGController(d)

	// stop the DAG
	wait := time.Millisecond * 500
	timer := time.Duration(0)
	timeout := resetartTimeout + d.MaxCleanUpTime

	st, err := c.GetStatus()
	if err != nil {
		return fmt.Errorf("restart failed because failed to get status: %v", err)
	}
	switch st.Status {
	case scheduler.SchedulerStatus_Running:
		log.Printf("Stopping %s for restart...", d.Name)
		for {
			st, err := c.GetStatus()
			if err != nil {
				log.Printf("Failed to get status: %v", err)
				continue
			}
			if st.Status == scheduler.SchedulerStatus_None {
				break
			}
			if err := c.Stop(); err != nil {
				return err
			}
			time.Sleep(wait)
			timer += wait
			if timer > timeout {
				return fmt.Errorf("restart failed because timeout")
			}
		}

		// wait for restartWaitTime
		log.Printf("wait for restart %s", d.RestartWait)
		time.Sleep(d.RestartWait)
	}

	// retrieve the parameter and start the DAG
	log.Printf("Restarting %s...", d.Name)
	st, err = c.GetLastStatus()
	if err != nil {
		return fmt.Errorf("failed to get the last status: %w", err)
	}
	params := st.Params
	d, err = loadDAG(ctx, ctx.Args().Get(0), params)
	if err != nil {
		return err
	}
	return start(d)
}
