package cli_test

import (
	"os"
	"os/exec"
	"testing"

	"strings"

	"github.com/dagu-org/dagu/internal/cli"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/spf13/cobra"
)

func TestValidateCommand(t *testing.T) {
	th := test.SetupCommand(t)

	t.Run("ValidSpec", func(t *testing.T) {
		dag := th.DAG(t, `
steps:
  - echo ok
`)

		// Run in subprocess to capture stdout/stderr and exit code
		sub := exec.Command(os.Args[0], "-test.run", "TestValidateCommand_HelperExit")
		sub.Env = append(os.Environ(),
			"RUN_VALIDATE_HELPER=1",
			"DAG_FILE_PATH="+dag.Location,
		)
		out, err := sub.CombinedOutput()
		if err != nil {
			t.Fatalf("expected exit code 0, got error: %v\nOutput: %s", err, string(out))
		}
		if !strings.Contains(string(out), "DAG spec is valid") {
			t.Fatalf("expected output to contain success message, got: %s", string(out))
		}
	})

	t.Run("InvalidDependencyExit1", func(t *testing.T) {
		// This DAG has a step depending on a non-existent step
		dagFile := th.CreateDAGFile(t, "invalid.yaml", `
steps:
  - echo A
  - name: "b"
    command: echo B
    depends: ["missing_step"]
`)

		// Re-run the current test binary in a subprocess to capture exit code
		cmdProc := exec.Command(os.Args[0], "-test.run", "TestValidateCommand_HelperExit")
		cmdProc.Env = append(os.Environ(),
			"RUN_VALIDATE_HELPER=1",
			"DAG_FILE_PATH="+dagFile,
		)
		out, err := cmdProc.CombinedOutput()
		if err == nil {
			t.Fatalf("expected process to exit with code 1, got success. Output: %s", string(out))
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() != 1 {
				t.Fatalf("expected exit code 1, got %d\nOutput: %s", exitErr.ExitCode(), string(out))
			}
		} else {
			t.Fatalf("unexpected error type: %T, %v", err, err)
		}
		if !strings.Contains(string(out), "Validation failed") {
			t.Fatalf("expected output to contain 'Validation failed', got: %s", string(out))
		}
	})
}

// Helper test to execute the validate command in a controlled subprocess.
func TestValidateCommand_HelperExit(t *testing.T) {
	if os.Getenv("RUN_VALIDATE_HELPER") != "1" {
		t.Skip("helper process")
		return
	}

	th := test.SetupCommand(t)
	dagFile := os.Getenv("DAG_FILE_PATH")

	root := &cobra.Command{Use: "root"}
	root.AddCommand(cli.CmdValidate())
	root.SetArgs([]string{"validate", dagFile})

	// This will os.Exit(1) on validation failure via NewCommand wrapper
	_ = root.ExecuteContext(th.Context)
}

// no extra helpers
