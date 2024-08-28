// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dag

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
		dg, err := Load("", filepath.Join(testdataDir, "default.yaml"), "")
		require.NoError(t, err)

		ret := dg.String()
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
