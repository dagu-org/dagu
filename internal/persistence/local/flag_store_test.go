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

package local

import (
	"os"
	"testing"

	"github.com/daguflow/dagu/internal/persistence/local/storage"

	"github.com/daguflow/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func TestFlagStore(t *testing.T) {
	tmpDir := util.MustTempDir("test-suspend-checker")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	flagStore := NewFlagStore(storage.NewStorage(tmpDir))

	require.False(t, flagStore.IsSuspended("test"))

	err := flagStore.ToggleSuspend("test", true)
	require.NoError(t, err)

	require.True(t, flagStore.IsSuspended("test"))
}
