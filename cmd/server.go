package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

const (
	defaultHost = "localhost"
	defaultPort = "8080"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "server",
		Short:   "Start the server",
		Long:    `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>] [--config=<config file>]`,
		PreRunE: bindServerFlags,
		RunE:    wrapRunE(runServer),
	}

	initServerFlags(cmd)
	return cmd
}

func bindServerFlags(cmd *cobra.Command, _ []string) error {
	return bindCommonFlags(cmd, []string{"port", "host", "dags"})
}

func runServer(cmd *cobra.Command, _ []string) error {
	setup, err := createSetup()
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	ctx := setup.loggerContext(cmd.Context(), false)

	logger.Info(ctx, "Server initialization", "host", setup.cfg.Host, "port", setup.cfg.Port)

	server, err := setup.server(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	if err := server.Serve(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func initServerFlags(cmd *cobra.Command) {
	initCommonFlags(cmd,
		[]commandLineFlag{
			{
				name:      "dags",
				shorthand: "d",
				usage:     "location of DAG files (default is $HOME/.config/dagu/dags)",
			},
			{
				name:         "host",
				shorthand:    "s",
				defaultValue: defaultHost,
				usage:        "server host",
			},
			{
				name:         "port",
				shorthand:    "p",
				defaultValue: defaultPort,
				usage:        "server port",
			},
		},
	)
}
