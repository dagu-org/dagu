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
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/dag/scheduler"
	"github.com/daguflow/dagu/internal/persistence/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-writer")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("WriteStatusToNewFile", func(t *testing.T) {
		statusFile := filepath.Join(tmpDir, "test_write_status.dat")
		w, err := newWriter(statusFile)
		require.NoError(t, err)
		defer w.close()

		d := &dag.DAG{
			Name:     "test_write_status",
			Location: "test_write_status.yaml",
		}
		status := model.NewStatus(d, nil, scheduler.StatusRunning, 10000, nil, nil)
		status.RequestID = "request-id-1"

		require.NoError(t, w.write(status))

		// Verify file contents
		dat, err := os.ReadFile(statusFile)
		require.NoError(t, err)

		var r model.Status
		err = json.Unmarshal(dat[:len(dat)-1], &r) // Remove trailing newline
		require.NoError(t, err)
		assert.Equal(t, d.Name, r.Name)
		assert.Equal(t, status.RequestID, r.RequestID)
		assert.Equal(t, scheduler.StatusRunning, r.Status)
	})

	t.Run("WriteStatusToExistingFile", func(t *testing.T) {
		statusFile := filepath.Join(tmpDir, "test_append_to_existing.dat")
		w, err := newWriter(statusFile)
		require.NoError(t, err)

		d := &dag.DAG{Name: "test_append_to_existing", Location: "test_append_to_existing.yaml"}
		initialStatus := model.NewStatus(d, nil, scheduler.StatusCancel, 10000, nil, nil)
		initialStatus.RequestID = "request-id-2"

		require.NoError(t, w.write(initialStatus))
		require.NoError(t, w.close())

		// Append to existing file
		w, err = newWriter(statusFile)
		require.NoError(t, err)
		updatedStatus := model.NewStatus(d, nil, scheduler.StatusSuccess, 10000, nil, nil)
		updatedStatus.RequestID = "request-id-2"
		require.NoError(t, w.write(updatedStatus))
		require.NoError(t, w.close())

		// Verify appended data
		dat, err := os.ReadFile(statusFile)
		require.NoError(t, err)

		lines := splitLines(string(dat))
		require.Len(t, lines, 2)

		var r1, r2 model.Status
		err = json.Unmarshal([]byte(lines[0]), &r1)
		require.NoError(t, err)
		err = json.Unmarshal([]byte(lines[1]), &r2)
		require.NoError(t, err)

		assert.Equal(t, scheduler.StatusCancel, r1.Status)
		assert.Equal(t, scheduler.StatusSuccess, r2.Status)
	})
}

func TestWriterErrorHandling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-writer-errors")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("OpenNonExistentDirectory", func(t *testing.T) {
		_, err := newWriter("/nonexistent/dir/file.dat")
		assert.Error(t, err)
	})

	t.Run("WriteToClosedWriter", func(t *testing.T) {
		w, err := newWriter(filepath.Join(tmpDir, "test.dat"))
		require.NoError(t, err)
		require.NoError(t, w.close())

		d := &dag.DAG{Name: "test", Location: "test.yaml"}
		status := model.NewStatus(d, nil, scheduler.StatusRunning, 10000, nil, nil)
		assert.Error(t, w.write(status))
	})

	t.Run("CloseMultipleTimes", func(t *testing.T) {
		w, err := newWriter(filepath.Join(tmpDir, "test.dat"))
		require.NoError(t, err)
		require.NoError(t, w.close())
		assert.NoError(t, w.close()) // Second close should not return an error
	})
}

func splitLines(s string) []string {
	var lines []string
	for len(s) > 0 {
		i := 0
		for i < len(s) && s[i] != '\n' {
			i++
		}
		lines = append(lines, s[:i])
		s = s[i+1:]
	}
	return lines
}
