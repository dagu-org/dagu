package logger

import (
	"bytes"
	"io"
	"log"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/dagu-dev/dagu/internal/utils"
)

func TestTeeLogger(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	tmpDir := utils.MustTempDir("test-tee-logger")
	tmpFile := path.Join(tmpDir, "test.log")
	f, err := os.Create(tmpFile)
	require.NoError(t, err)
	l := &Tee{Writer: f}
	err = l.Open()
	require.NoError(t, err)

	text := "test log"
	log.Println(text)
	os.Stdout = origStdout
	_ = w.Close()
	l.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	require.Contains(t, s, text)

	f, err = os.Open(tmpFile)
	require.NoError(t, err)
	b, err := io.ReadAll(f)
	require.NoError(t, err)
	require.Contains(t, string(b), text)
}
