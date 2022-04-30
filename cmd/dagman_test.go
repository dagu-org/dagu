package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagman/internal/settings"
	"github.com/yohamta/dagman/internal/utils"
)

type appTest struct {
	args        []string
	errored     bool
	output      []string
	exactOutput string
	stdin       io.ReadCloser
}

var testsDir = path.Join(utils.MustGetwd(), "../tests/testdata")

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("dagman_test")
	settings.InitTest(tempDir)
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func testConfig(name string) string {
	return path.Join(testsDir, name)
}

func runAppTestOutput(app *cli.App, test appTest, t *testing.T) {
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

	err = app.Run(test.args)
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
}

func runAppTest(app *cli.App, test appTest, t *testing.T) {
	err := app.Run(test.args)

	if err != nil && !test.errored {
		t.Fatalf("failed unexpectedly %v", err)
		return
	}
}
