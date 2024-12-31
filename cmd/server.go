// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/frontend"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/local"
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
		RunE:    wrapRunE(runServer),
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

	ctx := cmd.Context()
	ctx = logger.WithLogger(ctx, buildLogger(cfg, false))

	logger.Info(ctx, "Server initialization",
		"host", cfg.Host,
		"port", cfg.Port)

	dataStore := newDataStores(cfg)
	dagCache := filecache.New[*digraph.DAG](0, time.Hour*12)
	dagCache.StartEviction(ctx)
	dagStore := local.NewDAGStore(cfg.Paths.DAGsDir, local.WithFileCache(dagCache))

	cli := newClient(cfg, dataStore, dagStore)

	server := frontend.New(cfg, cli)
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
