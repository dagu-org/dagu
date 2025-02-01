package jsondb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testHelper struct {
	Context context.Context
	DB      *JSONDB
	tmpDir  string
}

func testSetup(t *testing.T) testHelper {
	tmpDir, err := os.MkdirTemp("", "test")
	require.NoError(t, err)

	th := testHelper{
		Context: context.Background(),
		DB:      New(tmpDir),
		tmpDir:  tmpDir,
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(th.tmpDir)
	})
	return th
}

func (th testHelper) DAG(name string) dagTestHelper {
	return dagTestHelper{
		th: th,
		DAG: &digraph.DAG{
			Name:     name,
			Location: filepath.Join(th.tmpDir, name+".yaml"),
		},
	}
}

type dagTestHelper struct {
	th testHelper
	*digraph.DAG
}

func (d dagTestHelper) Writer(t *testing.T, requestID string, startedAt time.Time) writerTestHelper {
	t.Helper()

	filePath, err := d.th.DB.generateFilePath(d.DAG.Location, newUTC(startedAt), requestID)
	require.NoError(t, err)

	writer := newWriter(filePath)
	require.NoError(t, writer.open())

	t.Cleanup(func() {
		require.NoError(t, writer.close())
	})

	return writerTestHelper{
		th: d.th,

		RequestID: requestID,
		FilePath:  filePath,
		Writer:    writer,
	}
}

func (w writerTestHelper) Write(t *testing.T, status model.Status) {
	t.Helper()

	err := w.Writer.write(status)
	require.NoError(t, err)
}

func (w writerTestHelper) AssertContent(t *testing.T, name, requestID string, status scheduler.Status) {
	t.Helper()

	data, err := ParseStatusFile(w.FilePath)
	require.NoError(t, err)

	assert.Equal(t, name, data.Name)
	assert.Equal(t, requestID, data.RequestID)
	assert.Equal(t, status, data.Status)
}

func (w writerTestHelper) Close(t *testing.T) {
	t.Helper()

	require.NoError(t, w.Writer.close())
}

type writerTestHelper struct {
	th testHelper

	RequestID string
	FilePath  string
	Writer    *writer
	Closed    bool
}
