package localhistory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type JSONDBTest struct {
	Context context.Context
	Repo    models.HistoryRepository
	tmpDir  string
}

func setupTestJSONDB(t *testing.T) JSONDBTest {
	tmpDir, err := os.MkdirTemp("", "test")
	require.NoError(t, err)

	th := JSONDBTest{
		Context: context.Background(),
		Repo:    New(tmpDir),
		tmpDir:  tmpDir,
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(th.tmpDir)
	})
	return th
}

func (th JSONDBTest) CreateRecord(t *testing.T, ts time.Time, requestID string, s scheduler.Status) *Record {
	t.Helper()

	dag := th.DAG("test_DAG")
	record, err := th.Repo.Create(th.Context, dag.DAG, ts, requestID, models.NewRecordOptions{})
	require.NoError(t, err)

	err = record.Open(th.Context)
	require.NoError(t, err)

	defer func() {
		_ = record.Close(th.Context)
	}()

	status := models.InitialStatus(dag.DAG)
	status.RequestID = requestID
	status.Status = s

	err = record.Write(th.Context, status)
	require.NoError(t, err)

	return record.(*Record)
}

func (th JSONDBTest) DAG(name string) DAGTest {
	return DAGTest{
		th: th,
		DAG: &digraph.DAG{
			Name:     name,
			Location: filepath.Join(th.tmpDir, name+".yaml"),
		},
	}
}

type DAGTest struct {
	th JSONDBTest
	*digraph.DAG
}

func (d DAGTest) Writer(t *testing.T, requestID string, startedAt time.Time) WriterTest {
	t.Helper()

	root := NewDataRoot(d.th.tmpDir, d.Name)
	run, err := root.CreateRun(NewUTC(startedAt), requestID)
	require.NoError(t, err)

	obj := d.th.Repo.(*historyStorage)
	record, err := run.CreateRecord(d.th.Context, NewUTC(startedAt), obj.cache, WithDAG(d.DAG))
	require.NoError(t, err)

	writer := NewWriter(record.file)
	require.NoError(t, writer.Open())

	t.Cleanup(func() {
		require.NoError(t, writer.close())
	})

	return WriterTest{
		th: d.th,

		RequestID: requestID,
		FilePath:  record.file,
		Writer:    writer,
	}
}

func (w WriterTest) Write(t *testing.T, status models.Status) {
	t.Helper()

	err := w.Writer.write(status)
	require.NoError(t, err)
}

func (w WriterTest) AssertContent(t *testing.T, name, requestID string, status scheduler.Status) {
	t.Helper()

	data, err := ParseStatusFile(w.FilePath)
	require.NoError(t, err)

	assert.Equal(t, name, data.Name)
	assert.Equal(t, requestID, data.RequestID)
	assert.Equal(t, status, data.Status)
}

func (w WriterTest) Close(t *testing.T) {
	t.Helper()

	require.NoError(t, w.Writer.close())
}

type WriterTest struct {
	th JSONDBTest

	RequestID string
	FilePath  string
	Writer    *Writer
	Closed    bool
}
