// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	dsclient "github.com/dagu-org/dagu/internal/persistence/client"
	"github.com/google/uuid"
)

func newClient(cfg *config.Config, ds persistence.DataStores, lg logger.Logger) client.Client {
	return client.New(ds, cfg.Executable, cfg.WorkDir, lg)
}

func newDataStores(cfg *config.Config) persistence.DataStores {
	return dsclient.NewDataStores(
		cfg.DAGs,
		cfg.DataDir,
		cfg.SuspendFlagsDir,
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
	Signal(os.Signal)
}

var signalChan = make(chan os.Signal, 100)

// listenSignals subscribes to the OS signals and passes them to the listener.
// It listens for the context cancellation as well.
func listenSignals(ctx context.Context, listener signalListener) {
	go func() {
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			listener.Signal(os.Interrupt)
		case sig := <-signalChan:
			listener.Signal(sig)
		}
	}()
}
