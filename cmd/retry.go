package main

import (
	"errors"
	"jobctl/internal/agent"
	"jobctl/internal/config"
	"jobctl/internal/database"
	"jobctl/internal/models"
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

func newRetryCommand() *cli.Command {
	cl := config.NewConfigLoader()
	return &cli.Command{
		Name:  "retry",
		Usage: "jobctl retry --req=<request-id> <config>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "req",
				Usage:    "request-id",
				Value:    "",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return errors.New("config file must be specified.")
			}
			if c.NArg() != 1 {
				return errors.New("too many parameters.")
			}
			config_file_path, err := filepath.Abs(c.Args().Get(0))
			if err != nil {
				return err
			}
			requestId := c.String("req")
			db := database.New(database.DefaultConfig())
			status, err := db.FindByRequestId(config_file_path, requestId)
			if err != nil {
				return err
			}
			cfg, err := cl.Load(config_file_path, status.Status.Params)
			if err != nil {
				return err
			}
			return retryJob(cfg, status.Status)
		},
	}
}

func retryJob(cfg *config.Config, status *models.Status) error {
	a := &agent.Agent{
		Config: &agent.Config{
			Job: cfg,
			Dry: false,
		},
		RetryConfig: &agent.RetryConfig{
			Status: status,
		},
	}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	err := a.Run()
	if err != nil {
		log.Printf("running job failed. %v", err)
	}
	return nil
}
