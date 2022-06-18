package cmd

import (
	"github.com/urfave/cli/v2"
)

func makeApp() *cli.App {
	return &cli.App{
		Name:      "Dagu",
		Usage:     "Self-contained, easy-to-use workflow engine for smaller use cases",
		UsageText: "dagu [options] <start|status|stop|retry|dry|server|version> [args]",
		Commands: []*cli.Command{
			newStartCommand(),
			newStatusCommand(),
			newStopCommand(),
			newRetryCommand(),
			newDryCommand(),
			newServerCommand(),
			newVersionCommand(),
		},
	}
}
