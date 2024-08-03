// Copyright (C) 2024 The Daguflow/Dagu Authors
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

package storage

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/daguflow/dagu/internal/util"
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
