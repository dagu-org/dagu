// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/core"
)

func TestPreResolveEnvSecrets(t *testing.T) {
	t.Setenv("MY_REAL_SECRET", "s3cret")

	refs := []core.SecretRef{
		{Name: "SECRET_A", Provider: "env", Key: "MY_REAL_SECRET"},
		{Name: "SECRET_B", Provider: "env", Key: "NONEXISTENT_VAR"},
		{Name: "SECRET_C", Provider: "file", Key: "/path/to/file"},
	}
	result := preResolveEnvSecrets(refs)
	require.Len(t, result, 1)
	require.Equal(t, "_DAGU_PRESOLVED_SECRET_MY_REAL_SECRET=s3cret", result[0])
}

func TestPreResolveEnvSecrets_NilRefs(t *testing.T) {
	result := preResolveEnvSecrets(nil)
	require.Nil(t, result)
}

func TestPreResolveEnvSecrets_EmptyRefs(t *testing.T) {
	result := preResolveEnvSecrets([]core.SecretRef{})
	require.Nil(t, result)
}
