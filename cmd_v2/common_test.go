package cmd_v2

import (
	"bytes"
	"io"
	"log"
	"os"
	"path"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/utils"
)

type cmdTest struct {
	args        []string
	expectedOut []string
	expectedErr []string
}

func testRunCommand(t *testing.T, cmd *cobra.Command, test cmdTest) {
	t.Helper()

	root := &cobra.Command{Use: "root"}
	root.AddCommand(cmd)

	// Set arguments.
	root.SetArgs(test.args)

	// Run the command.
	out := withSpool(t, func() {
		err := root.Execute()
		require.NoError(t, err)
	})

	// Check outputs.
	for _, s := range test.expectedOut {
		require.Contains(t, out, s)
	}
}

func withSpool(t *testing.T, f func()) string {
	t.Helper()

	origStdout := os.Stdout

	r, w, err := os.Pipe()
	require.NoError(t, err)

	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
		w.Close()
	}()

	f()

	os.Stdout = origStdout
	w.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	return buf.String()
}

func testConfig(name string) string {
	d := path.Join(utils.MustGetwd(), "testdata")
	return path.Join(d, name)
}
