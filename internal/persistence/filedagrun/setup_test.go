package filedagrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type StoreTest struct {
	Context context.Context
	Store   models.DAGRunStore
	TmpDir  string
}

func setupTestStore(t *testing.T) StoreTest {
	tmpDir, err := os.MkdirTemp("", "test")
	require.NoError(t, err)

	th := StoreTest{
		Context: context.Background(),
		Store:   New(tmpDir),
		TmpDir:  tmpDir,
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(th.TmpDir)
	})
	return th
}

func (th StoreTest) CreateAttempt(t *testing.T, ts time.Time, dagRunID string, s status.Status) *Attempt {
	t.Helper()

	dag := th.DAG("test_DAG")
	attempt, err := th.Store.CreateAttempt(th.Context, dag.DAG, ts, dagRunID, models.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	err = attempt.Open(th.Context)
	require.NoError(t, err)

	defer func() {
		_ = attempt.Close(th.Context)
	}()

	dagRunStatus := models.InitialStatus(dag.DAG)
	dagRunStatus.DAGRunID = dagRunID
	dagRunStatus.Status = s

	err = attempt.Write(th.Context, dagRunStatus)
	require.NoError(t, err)

	return attempt.(*Attempt)
}

func (th StoreTest) DAG(name string) DAGTest {
	return DAGTest{
		th: th,
		DAG: &digraph.DAG{
			Name:     name,
			Location: filepath.Join(th.TmpDir, name+".yaml"),
		},
	}
}

type DAGTest struct {
	th StoreTest
	*digraph.DAG
}

func (d DAGTest) Writer(t *testing.T, dagRunID string, startedAt time.Time) WriterTest {
	t.Helper()

	root := NewDataRoot(d.th.TmpDir, d.Name)
	dagRun, err := root.CreateDAGRun(models.NewUTC(startedAt), dagRunID)
	require.NoError(t, err)

	store := d.th.Store.(*Store)
	attempt, err := dagRun.CreateAttempt(d.th.Context, models.NewUTC(startedAt), store.cache, WithDAG(d.DAG))
	require.NoError(t, err)

	writer := NewWriter(attempt.file)
	require.NoError(t, writer.Open())

	t.Cleanup(func() {
		require.NoError(t, writer.close())
	})

	return WriterTest{
		th: d.th,

		DAGRunID: dagRunID,
		FilePath: attempt.file,
		Writer:   writer,
	}
}

func (w WriterTest) Write(t *testing.T, dagRunStatus models.DAGRunStatus) {
	t.Helper()

	err := w.Writer.write(dagRunStatus)
	require.NoError(t, err)
}

func (w WriterTest) AssertContent(t *testing.T, name, dagRunID string, st status.Status) {
	t.Helper()

	data, err := ParseStatusFile(w.FilePath)
	require.NoError(t, err)

	assert.Equal(t, name, data.Name)
	assert.Equal(t, dagRunID, data.DAGRunID)
	assert.Equal(t, st, data.Status)
}

func (w WriterTest) Close(t *testing.T) {
	t.Helper()

	require.NoError(t, w.Writer.close())
}

type WriterTest struct {
	th StoreTest

	DAGRunID string
	FilePath string
	Writer   *Writer
	Closed   bool
}
