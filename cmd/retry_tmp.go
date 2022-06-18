package cmd

import (
	"path/filepath"

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
