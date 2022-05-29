package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/constants"
)

func newVersionCommand() *cli.Command {
	return &cli.Command{
		Name: "version",
		Usage: "dagu version",
		Action: func(c *cli.Context) error {
			if constants.Version != "" {
				fmt.Println(constants.Version)
				return nil
			}
			return fmt.Errorf("failed to get version")
		},
	}
}
