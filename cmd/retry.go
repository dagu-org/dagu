package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/utils"

	"github.com/urfave/cli/v2"
)

func newRetryCommand() *cli.Command {
	cl := &config.Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
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
			return retry(cfg, status.Status)
		},
	}
}

func retry(cfg *config.Config, status *models.Status) error {
	a := &agent.Agent{
		Config: &agent.Config{
			DAG: cfg,
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
		log.Printf("running failed. %v", err)
	}
	return nil
}
