package agent

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func TestTeeWriter(t *testing.T) {
	t.Run("It should tee the log to the file", func(t *testing.T) {
		// Redirect stdout to a pipe to capture the log.
		origStdout := os.Stdout
		r, w, err := os.Pipe()
		require.NoError(t, err)
		os.Stdout = w
		log.SetOutput(w)

		defer func() {
			os.Stdout = origStdout
			log.SetOutput(origStdout)
		}()

		// Create a temporary file and tee the log to the file.
		tmpLogFile := filepath.Join(util.MustTempDir("test-tee"), "test.log")
		logFile, err := os.Create(tmpLogFile)
		require.NoError(t, err)

		tw := &teeWriter{Writer: logFile}
		err = tw.Open()
		require.NoError(t, err)

		// Write a log.
		log.Println("test log")

		// Reset the log output.
		os.Stdout = origStdout

		// Close the writer.
		_ = w.Close()
		tw.Close()

		// Check the log is written to stdout.
		var stdoutBuf bytes.Buffer
		_, err = io.Copy(&stdoutBuf, r)
		require.NoError(t, err)
		require.Contains(t, stdoutBuf.String(), "test log")

		// Check the log is written to the file.
		logFile, err = os.Open(tmpLogFile)
		require.NoError(t, err)

		logData, err := io.ReadAll(logFile)
		require.NoError(t, err)
		require.Contains(t, string(logData), "test log")
	})
}
