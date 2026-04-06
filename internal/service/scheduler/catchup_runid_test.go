// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCatchupRunID(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 12, 14, 0, 0, 0, time.UTC)

	t.Run("normal name", func(t *testing.T) {
		id := GenerateCatchupRunID("etl-pipeline", ts)
		assert.Contains(t, id, "catchup-etl-pipeline-")
		assert.Contains(t, id, "-20260312T140000")
		require.NoError(t, exec.ValidateDAGRunID(id))
	})

	t.Run("deterministic", func(t *testing.T) {
		id1 := GenerateCatchupRunID("my-dag", ts)
		id2 := GenerateCatchupRunID("my-dag", ts)
		assert.Equal(t, id1, id2)
	})

	t.Run("different timestamps produce different IDs", func(t *testing.T) {
		ts2 := ts.Add(time.Hour)
		id1 := GenerateCatchupRunID("my-dag", ts)
		id2 := GenerateCatchupRunID("my-dag", ts2)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("dots replaced with underscores", func(t *testing.T) {
		id := GenerateCatchupRunID("my.dag.name", ts)
		assert.Contains(t, id, "my_dag_name")
		require.NoError(t, exec.ValidateDAGRunID(id))
	})

	t.Run("dot vs hyphen produce different IDs", func(t *testing.T) {
		id1 := GenerateCatchupRunID("my.dag", ts)
		id2 := GenerateCatchupRunID("my-dag", ts)
		assert.NotEqual(t, id1, id2, "dot and hyphen DAG names must produce different IDs due to hash")
	})

	t.Run("dot vs underscore produce different IDs", func(t *testing.T) {
		id1 := GenerateCatchupRunID("my.dag", ts)
		id2 := GenerateCatchupRunID("my_dag", ts)
		assert.NotEqual(t, id1, id2, "dot and underscore DAG names must produce different IDs due to hash")
	})

	t.Run("long name truncated", func(t *testing.T) {
		longName := "a-very-extremely-long-dag-name-that-exceeds-the-limit"
		id := GenerateCatchupRunID(longName, ts)
		assert.LessOrEqual(t, len(id), maxRunIDLen)
		require.NoError(t, exec.ValidateDAGRunID(id))
	})

	t.Run("max length exactly 64", func(t *testing.T) {
		// maxNameLen = 64 - 8 - 1 - 8 - 1 - 15 = 31
		name := "abcdefghijklmnopqrstuvwxyz12345" // 31 chars
		id := GenerateCatchupRunID(name, ts)
		assert.LessOrEqual(t, len(id), maxRunIDLen)
		require.NoError(t, exec.ValidateDAGRunID(id))
	})

	t.Run("all outputs pass validation", func(t *testing.T) {
		names := []string{
			"simple",
			"with-hyphens",
			"with_underscores",
			"with.dots",
			"MixedCase",
			"a",
			"a-very-extremely-long-dag-name-that-definitely-exceeds",
		}
		for _, name := range names {
			id := GenerateCatchupRunID(name, ts)
			require.NoError(t, exec.ValidateDAGRunID(id), "failed for DAG name: %s, generated ID: %s", name, id)
		}
	})

	t.Run("UTC normalization", func(t *testing.T) {
		loc, _ := time.LoadLocation("America/New_York")
		tsLocal := ts.In(loc)
		id1 := GenerateCatchupRunID("my-dag", ts)
		id2 := GenerateCatchupRunID("my-dag", tsLocal)
		assert.Equal(t, id1, id2, "same instant in different timezones must produce same ID")
	})
}

func TestGenerateOneOffRunID(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 3, 29, 1, 10, 0, 0, time.UTC)
	fingerprint := "at:2026-03-29T02:10:00+01:00"

	t.Run("normal name", func(t *testing.T) {
		id := GenerateOneOffRunID("etl-pipeline", fingerprint, ts)
		assert.Contains(t, id, "oneoff-etl-pipeline-")
		assert.Contains(t, id, "-20260329T011000")
		require.NoError(t, exec.ValidateDAGRunID(id))
	})

	t.Run("deterministic", func(t *testing.T) {
		id1 := GenerateOneOffRunID("my-dag", fingerprint, ts)
		id2 := GenerateOneOffRunID("my-dag", fingerprint, ts)
		assert.Equal(t, id1, id2)
	})

	t.Run("fingerprint changes ID", func(t *testing.T) {
		id1 := GenerateOneOffRunID("my-dag", fingerprint, ts)
		id2 := GenerateOneOffRunID("my-dag", "at:2026-03-29T02:11:00+01:00", ts)
		assert.NotEqual(t, id1, id2)
	})

	t.Run("UTC normalization", func(t *testing.T) {
		loc, _ := time.LoadLocation("America/New_York")
		tsLocal := ts.In(loc)
		id1 := GenerateOneOffRunID("my-dag", fingerprint, ts)
		id2 := GenerateOneOffRunID("my-dag", fingerprint, tsLocal)
		assert.Equal(t, id1, id2)
	})
}
