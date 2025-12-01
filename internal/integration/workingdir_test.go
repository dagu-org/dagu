package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestWorkingDirectoryResolution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - uses bash commands")
	}

	th := test.Setup(t)

	// Create directory structure
	tempDir := t.TempDir()
	parentScripts := filepath.Join(tempDir, "parent_scripts")
	childScripts := filepath.Join(tempDir, "child_scripts")
	require.NoError(t, os.MkdirAll(parentScripts, 0755))
	require.NoError(t, os.MkdirAll(childScripts, 0755))

	dag := th.DAG(t, `
workingDir: `+parentScripts+`
steps:
  - name: parent_pwd
    command: pwd
    output: PARENT_DIR
  - name: parent_relative_step
    dir: ../child_scripts
    command: pwd
    output: PARENT_STEP_DIR
  - name: call_child_with_wd
    call: child_with_wd
  - name: call_child_no_wd
    call: child_no_wd

---

name: child_with_wd
workingDir: `+childScripts+`
steps:
  - name: child_pwd
    command: pwd
    output: CHILD_DIR

---

name: child_no_wd
steps:
  - name: child_inherited_pwd
    command: pwd
    output: CHILD_INHERITED_DIR
`)

	agent := dag.Agent()
	agent.RunSuccess(t)

	dag.AssertOutputs(t, map[string]any{
		"PARENT_DIR":      parentScripts,
		"PARENT_STEP_DIR": childScripts,
	})
}
