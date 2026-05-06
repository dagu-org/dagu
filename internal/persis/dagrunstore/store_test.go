// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dagrunstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagucloud/dagu/internal/cmn/config"
)

func TestNewPostgresRequiresRoleSpecificDSN(t *testing.T) {
	cfg := &config.Config{
		DAGRunStore: config.DAGRunStoreConfig{
			Backend: config.DAGRunStoreBackendPostgres,
			Postgres: config.DAGRunStorePostgresConfig{
				Server: config.DAGRunStorePostgresRoleConfig{
					DSN: "postgres://server@example.invalid/dagu",
				},
				Scheduler: config.DAGRunStorePostgresRoleConfig{
					DSN: "postgres://scheduler@example.invalid/dagu",
				},
			},
		},
	}

	_, err := New(context.Background(), cfg, WithRole(RoleAgent))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "dag_run_store.postgres.agent.dsn is required")
}
