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

package jsondb

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/daguflow/dagu/internal/persistence/model"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/dag/scheduler"
	"github.com/stretchr/testify/require"
)

func TestWriteStatusToFile(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := &dag.DAG{
		Name:     "test_write_status",
		Location: "test_write_status.yaml",
	}
	dw, file, err := te.JSONDB.newWriter(d.Location, time.Now(), "request-id-1")
	require.NoError(t, err)
	require.NoError(t, dw.open())
	defer func() {
		_ = dw.close()
		_ = te.JSONDB.RemoveOld(d.Location, 0)
	}()

	status := model.NewStatus(d, nil, scheduler.StatusRunning, 10000, nil, nil)
	status.RequestID = fmt.Sprintf("request-id-%d", time.Now().Unix())
	require.NoError(t, dw.write(status))
	require.Regexp(t, ".*test_write_status.*", file)

	dat, err := os.ReadFile(file)
	require.NoError(t, err)

	r, err := model.StatusFromJSON(string(dat))
	require.NoError(t, err)

	require.Equal(t, d.Name, r.Name)

	err = dw.close()
	require.NoError(t, err)

	// TODO: fixme
	oldS := d.Location
	newS := "text_write_status_new.yaml"

	oldDir := te.JSONDB.getDirectory(oldS, prefix(oldS))
	newDir := te.JSONDB.getDirectory(newS, prefix(newS))
	require.DirExists(t, oldDir)
	require.NoDirExists(t, newDir)

	err = te.JSONDB.Rename(oldS, newS)
	require.NoError(t, err)
	require.NoDirExists(t, oldDir)
	require.DirExists(t, newDir)

	ret := te.JSONDB.ReadStatusRecent(newS, 1)
	require.Equal(t, 1, len(ret))
	require.Equal(t, status.RequestID, ret[0].Status.RequestID)
}

func TestWriteStatusToExistingFile(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := &dag.DAG{Name: "test_append_to_existing", Location: "test_append_to_existing.yaml"}
	dw, file, err := te.JSONDB.newWriter(d.Location, time.Now(), "request-id-1")
	require.NoError(t, err)
	require.NoError(t, dw.open())

	status := model.NewStatus(d, nil, scheduler.StatusCancel, 10000, nil, nil)
	status.RequestID = "request-id-test-write-status-to-existing-file"
	require.NoError(t, dw.write(status))
	_ = dw.close()

	data, err := te.JSONDB.FindByRequestID(d.Location, status.RequestID)
	require.NoError(t, err)
	require.Equal(t, data.Status.Status, scheduler.StatusCancel)
	require.Equal(t, file, data.File)

	dw = &writer{target: file}
	require.NoError(t, dw.open())
	status.Status = scheduler.StatusSuccess
	require.NoError(t, dw.write(status))
	_ = dw.close()

	data, err = te.JSONDB.FindByRequestID(d.Location, status.RequestID)
	require.NoError(t, err)
	require.Equal(t, data.Status.Status, scheduler.StatusSuccess)
	require.Equal(t, file, data.File)
}
