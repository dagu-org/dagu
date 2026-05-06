// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package postgres

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/persis/dagrunstore/postgres/migrations"
)

func TestMigrationUsesExistingIdentifierConstraints(t *testing.T) {
	data, err := migrations.FS.ReadFile("20260506000000_create_dag_run_attempts.sql")
	require.NoError(t, err)

	sql := string(data)
	assert.Contains(t, sql, "VALUE ~ '^[a-zA-Z0-9_.-]+$'")
	assert.Contains(t, sql, "char_length(VALUE) <= 40")
	assert.Contains(t, sql, "VALUE ~ '^[-a-zA-Z0-9_]+$'")
	assert.Contains(t, sql, "char_length(VALUE) <= 64")
	assert.Contains(t, sql, "VALUE ~ '^[A-Za-z0-9_-]+$'")
	assert.Contains(t, sql, "lower(VALUE) NOT IN ('all', 'default')")
	assert.Contains(t, sql, "VALUE::text ~* '^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$'")
}

func TestWorkspaceFromLabels(t *testing.T) {
	t.Run("Missing", func(t *testing.T) {
		workspaceName, valid := workspaceFromLabels(core.NewLabels(nil))
		assert.Equal(t, sql.NullString{}, workspaceName)
		assert.True(t, valid)
	})

	t.Run("Valid", func(t *testing.T) {
		workspaceName, valid := workspaceFromLabels(core.NewLabels([]string{"workspace=ops"}))
		assert.Equal(t, sql.NullString{String: "ops", Valid: true}, workspaceName)
		assert.True(t, valid)
	})

	t.Run("Invalid", func(t *testing.T) {
		workspaceName, valid := workspaceFromLabels(core.NewLabels([]string{"workspace=default"}))
		assert.Equal(t, sql.NullString{}, workspaceName)
		assert.False(t, valid)
	})
}
