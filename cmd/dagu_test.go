package cmd

import (
	"bytes"
	"io"
	"log"
	"os"
	"path"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

type appTest struct {
	args        []string
	flags       map[string]string
	errored     bool
	output      []string
	exactOutput string
	stdin       io.ReadCloser
}

var testsDir = path.Join(utils.MustGetwd(), "../tests/testdata")

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("dagu_test")
	settings.ChangeHomeDir(tempDir)
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func TestSetVersion(t *testing.T) {
	version = "0.0.1"
	setVersion()
	require.Equal(t, version, constants.Version)
}

func testConfig(name string) string {
	return path.Join(testsDir, name)
}

func runCmdTestOutput(cmd *cobra.Command, test appTest, t *testing.T) {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	log.SetOutput(w)

	defer func() {
		os.Stdout = origStdout
		log.SetOutput(origStdout)
	}()

	if test.stdin != nil {
		origStdin := stdin
		stdin = test.stdin
		defer func() {
			stdin = origStdin
		}()
	}

	for k, v := range test.flags {
		cmd.Flag(k).Value.Set(v)
	}
	err = cmd.RunE(nil, test.args)
	os.Stdout = origStdout
	w.Close()

	if err != nil && !test.errored {
		t.Fatalf("failed unexpectedly %v", err)
		return
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	s := buf.String()
	if len(test.output) > 0 {
		for _, v := range test.output {
			require.Contains(t, s, v)
		}
	}

	if test.exactOutput != "" {
		require.Equal(t, test.exactOutput, s)
	}

	// TODO: 掃除。より良い方法がないか
	for k := range test.flags {
		cmd.Flag(k).Value.Set("")
	}
}

func runCmdTest(cmd *cobra.Command, test appTest, t *testing.T) {
	err := cmd.RunE(nil, test.args)

	if err != nil && !test.errored {
		t.Fatalf("failed unexpectedly %v", err)
		return
	}
}
