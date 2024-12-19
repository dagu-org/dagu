// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"log"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
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

			quiet, err := cmd.Flags().GetBool("quiet")
			if err != nil {
				log.Fatalf("Flag retrieval failed (quiet): %v", err)
			}

			logger := buildLogger(cfg, quiet)

			dag, err := digraph.Load(cmd.Context(), cfg.BaseConfig, args[0], "")
			if err != nil {
				logger.Fatal("DAG load failed", "error", err, "file", args[0])
			}

			logger.Info("DAG stop initiated", "DAG", dag.Name)

			dataStore := newDataStores(cfg)
			cli := newClient(cfg, dataStore, logger)

			if err := cli.Stop(cmd.Context(), dag); err != nil {
				logger.Fatal(
					"DAG stop operation failed",
					"error",
					err,
					"dag",
					dag.Name,
				)
			}
		},
	}
	cmd.Flags().BoolP("quiet", "q", false, "suppress output")
	return cmd
}
