// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandHomeDir(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	require.NoError(t, err)

	assert.Equal(t, homeDir, expandHomeDir("~"))
	assert.Equal(t, filepath.Join(homeDir, "dags", "test.yaml"), expandHomeDir("~/dags/test.yaml"))
	assert.Equal(t, "~alice/dags/test.yaml", expandHomeDir("~alice/dags/test.yaml"))
}
