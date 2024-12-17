// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/util"
)

func TestStorage(t *testing.T) {
	tmpDir := util.MustTempDir("test-storage")
	defer os.RemoveAll(tmpDir)

	storage := NewStorage(tmpDir)

	f := "test.flag"
	exist := storage.Exists(f)
	require.False(t, exist)

	err := storage.Create(f)
	require.NoError(t, err)

	exist = storage.Exists(f)
	require.True(t, exist)

	err = storage.Delete(f)
	require.NoError(t, err)

	exist = storage.Exists(f)
	require.False(t, exist)
}
