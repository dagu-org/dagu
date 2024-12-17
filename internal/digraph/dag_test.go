// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

var (
	testdataDir = filepath.Join(util.MustGetwd(), "testdata")
)

func TestDAG_String(t *testing.T) {
	t.Run("DefaltConfig", func(t *testing.T) {
		dag, err := Load("", filepath.Join(testdataDir, "default.yaml"), "")
		require.NoError(t, err)

		ret := dag.String()
		require.Contains(t, ret, "Name: default")
	})
}

func TestDAG_SockAddr(t *testing.T) {
	t.Run("UnixSocketLocation", func(t *testing.T) {
		workflow := &DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu-testDag-[0-9a-f]+\.sock$`, workflow.SockAddr())
	})
	t.Run("MaxUnixSocketLength", func(t *testing.T) {
		workflow := &DAG{
			Location: "testdata/testDagVeryLongNameThatExceedsUnixSocketLengthMaximum-testDagVeryLongNameThatExceedsUnixSocketLengthMaximum.yml",
		}
		// 108 is the maximum length of a unix socket address
		require.Greater(t, 108, len(workflow.SockAddr()))
		require.Equal(
			t,
			"/tmp/@dagu-testDagVeryLongNameThatExceedsUnixSocketLengthMax-b92b711162d6012f025a76d0cf0b40c2.sock",
			workflow.SockAddr(),
		)
	})
}
