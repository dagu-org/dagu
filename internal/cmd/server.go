package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdServer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		Long:  `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>] [--config=<config file>]`,
		RunE:  wrapRunE(runServer),
	}
	initFlags(cmd, dagsFlag, hostFlag, portFlag)
	return cmd
}

func runServer(cmd *cobra.Command, _ []string) error {
	setup, err := createSetup(cmd.Context(), false)
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	ctx := setup.ctx
	logger.Info(ctx, "Server initialization", "host", setup.cfg.Server.Host, "port", setup.cfg.Server.Port)

	server, err := setup.server(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	if err := server.Serve(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
