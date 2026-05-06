// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package postgres

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetentionQueriesUseLatestAttemptLikeFileStore(t *testing.T) {
	queryText := readAttemptQueries(t)

	daysQuery := namedQuery(t, queryText, "ListRemovableRunsByDays")
	assert.NotContains(t, latestCTE(t, daysQuery), "status_data IS NOT NULL")
	assert.Contains(t, daysQuery, "status_data IS NOT NULL")
	assert.Contains(t, daysQuery, "updated_at < sqlc.arg(cutoff)::timestamptz")

	countQuery := namedQuery(t, queryText, "ListRemovableRunsByCount")
	assert.NotContains(t, latestCTE(t, countQuery), "status_data IS NOT NULL")
	assert.Contains(t, countQuery, "status_data IS NOT NULL")
}

func TestDeleteDAGRunRowsReturnsDistinctRunIDs(t *testing.T) {
	query := namedQuery(t, readAttemptQueries(t), "DeleteDAGRunRows")

	assert.Contains(t, query, "WITH deleted AS (")
	assert.Contains(t, query, "SELECT DISTINCT dag_run_id")
	assert.Contains(t, query, "ORDER BY dag_run_id")
}

func TestRenameDAGRunsUpdatesRootStatusName(t *testing.T) {
	query := namedQuery(t, readAttemptQueries(t), "RenameDAGRuns")

	assert.Contains(t, query, "status_data = CASE")
	assert.Contains(t, query, "WHEN is_root")
	assert.Contains(t, query, "jsonb_set(status_data, '{name}', to_jsonb(sqlc.arg(new_name)::text), true)")
}

func readAttemptQueries(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("queries/attempts.sql")
	require.NoError(t, err)
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func namedQuery(t *testing.T, queryText, name string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?ms)-- name: ` + regexp.QuoteMeta(name) + ` :\w+\n(.*?)(?:\n-- name:|\z)`)
	matches := pattern.FindStringSubmatch(queryText)
	require.Len(t, matches, 2)
	return strings.TrimSpace(matches[1])
}

func latestCTE(t *testing.T, query string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?ms)WITH latest AS \(\n(.*?)\n\)`)
	matches := pattern.FindStringSubmatch(query)
	require.Len(t, matches, 2)
	return matches[1]
}
