// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package buildenv

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareAndLoad(t *testing.T) {
	extraEnv, cleanup, err := Prepare([]string{
		"SECOND=value-2",
		"FIRST=value-1",
		"SECOND=latest",
	})
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	t.Cleanup(func() { require.NoError(t, cleanup()) })
	require.Len(t, extraEnv, 1)

	key, path, ok := strings.Cut(extraEnv[0], "=")
	require.True(t, ok)
	require.Equal(t, PresolvedEnvFileKey, key)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	t.Setenv(PresolvedEnvFileKey, path)

	loaded, err := Load()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"FIRST":  "value-1",
		"SECOND": "latest",
	}, loaded)
}

func TestPrepare_EmptyEnv(t *testing.T) {
	t.Parallel()

	extraEnv, cleanup, err := Prepare(nil)
	require.NoError(t, err)
	assert.Nil(t, extraEnv)
	assert.Nil(t, cleanup)
}
