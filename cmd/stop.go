// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"log"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop /path/to/spec.yaml",
		Short: "Stop the running DAG",
		Long:  `dagu stop /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Load()
			if err != nil {
				log.Fatalf("Configuration load failed: %v", err)
			}

			ctx := cmd.Context()
			ctx = logger.WithLogger(ctx, buildLogger(cfg))

			dag, err := digraph.Load(cmd.Context(), cfg.Paths.BaseConfig, args[0], "")
			if err != nil {
				logger.Fatal(ctx, "DAG load failed", "error", err, "file", args[0])
			}

			logger.Info(ctx, "DAG is stopping", "dag", dag.Name)

			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore)

			if err := cli.Stop(cmd.Context(), dag); err != nil {
				logger.Fatal(ctx, "DAG stop operation failed", "error", err, "dag", dag.Name)
			}

			logger.Info(ctx, "DAG stopped", "dag", dag.Name)
		},
	}
	return cmd
}
