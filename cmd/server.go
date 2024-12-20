// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	defaultHost = "localhost"
	defaultPort = "8080"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "server",
		Short:   "Start the server",
		Long:    `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		PreRunE: bindFlags,
		RunE:    runServer,
	}

	initServerFlags(cmd)
	return cmd
}

func bindFlags(cmd *cobra.Command, _ []string) error {
	flags := []string{"port", "host", "dags"}
	for _, flag := range flags {
		if err := viper.BindPFlag(flag, cmd.Flags().Lookup(flag)); err != nil {
			return fmt.Errorf("failed to bind flag %s: %w", flag, err)
		}
	}
	return nil
}

func runServer(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger := buildLogger(cfg, false)
	logger.Info("Server initialization",
		"host", cfg.Host,
		"port", cfg.Port)

	dataStore := newDataStores(cfg)
	cli := newClient(cfg, dataStore, logger)

	server := frontend.New(cfg, logger, cli)
	if err := server.Serve(cmd.Context()); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func initServerFlags(cmd *cobra.Command) {
	flags := []struct {
		name, shorthand, defaultValue, usage string
	}{
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
	}

	for _, flag := range flags {
		cmd.Flags().StringP(
			flag.name,
			flag.shorthand,
			flag.defaultValue,
			flag.usage,
		)
	}
}
