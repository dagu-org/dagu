// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/persistence"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

func wrapRunE(f func(cmd *cobra.Command, args []string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if err := f(cmd, args); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return nil
	}
}

func newClient(cfg *config.Config, ds persistence.DataStores) client.Client {
	return client.New(ds, cfg.Paths.Executable, cfg.WorkDir)
}

func newDataStores(cfg *config.Config) persistence.DataStores {
	return dsclient.NewDataStores(
		cfg.Paths.DAGsDir,
		cfg.Paths.DataDir,
		cfg.Paths.SuspendFlagsDir,
		dsclient.DataStoreOptions{
			LatestStatusToday: cfg.LatestStatusToday,
		},
	)
}

// generateRequestID generates a new request ID.
// For simplicity, we use UUIDs as request IDs.
func generateRequestID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

type signalListener interface {
	Signal(context.Context, os.Signal)
}

var signalChan = make(chan os.Signal, 100)

// listenSignals subscribes to the OS signals and passes them to the listener.
// It listens for the context cancellation as well.
func listenSignals(ctx context.Context, listener signalListener) {
	go func() {
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			listener.Signal(ctx, os.Interrupt)
		case sig := <-signalChan:
			listener.Signal(ctx, sig)
		}
	}()
}
