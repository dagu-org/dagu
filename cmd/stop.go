package main

import (
	"errors"
	"jobctl/internal/config"
	"jobctl/internal/controller"
	"jobctl/internal/scheduler"
	"log"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
)

func newStopCommand() *cli.Command {
	cl := config.NewConfigLoader()
	return &cli.Command{
		Name:  "stop",
		Usage: "jobctl stop <config>",
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return errors.New("config file must be specified.")
			}
			config_file_path := c.Args().Get(0)
			cfg, err := cl.Load(config_file_path, "")
			if err != nil {
				return err
			}
			return stopJob(cfg)
		},
	}
}

func stopJob(cfg *config.Config) error {
	status, err := controller.New(cfg).GetStatus()
	if err != nil {
		return err
	}

	if status.Status != scheduler.SchedulerStatus_Running ||
		!status.Pid.IsRunning() {
		log.Printf("job is not running.")
		return nil
	}
	syscall.Kill(int(status.Pid), syscall.SIGINT)
	for {
		time.Sleep(time.Second * 3)
		s, err := controller.New(cfg).GetStatus()
		if err != nil {
			return err
		}
		if s.Pid == status.Pid && s.Status ==
			scheduler.SchedulerStatus_Running {
			continue
		}
		break
	}
	log.Printf("job is stopped.")
	return nil
}
