// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package templatefuncs

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuncMapJoinRejectsUnsupportedInput(t *testing.T) {
	t.Parallel()

	tmpl, err := template.New("test").Funcs(FuncMap()).Parse(`{{ . | join "," }}`)
	require.NoError(t, err)

	var out bytes.Buffer
	err = tmpl.Execute(&out, map[string]string{"a": "b"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "join: unsupported type map[string]string")
	assert.Empty(t, out.String())
}

func TestFuncMapCountNilIsZero(t *testing.T) {
	t.Parallel()

	tmpl, err := template.New("test").Funcs(FuncMap()).Parse(`{{ count . }}`)
	require.NoError(t, err)

	var out bytes.Buffer
	require.NoError(t, tmpl.Execute(&out, nil))
	assert.Equal(t, "0", out.String())
}
