package scheduler

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/stretchr/testify/require"
)

func TestExecute(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         "true",
			OutputVariables: &dag.SyncMap{},
		}}
	require.NoError(t, n.Execute(context.Background()))
	require.Nil(t, n.Error)
}

func TestError(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         "false",
			OutputVariables: &dag.SyncMap{},
		}}
	err := n.Execute(context.Background())
	require.True(t, err != nil)
	require.Equal(t, n.Error, err)
}

func TestSignal(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         "sleep",
			Args:            []string{"100"},
			OutputVariables: &dag.SyncMap{},
		}}

	go func() {
		time.Sleep(100 * time.Millisecond)
		n.signal(syscall.SIGTERM, false)
	}()

	n.setStatus(NodeStatusRunning)
	err := n.Execute(context.Background())

	require.Error(t, err)
	require.Equal(t, n.State().Status, NodeStatusCancel)
}

func TestSignalSpecified(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         "sleep",
			Args:            []string{"100"},
			OutputVariables: &dag.SyncMap{},
			SignalOnStop:    "SIGINT",
		}}

	go func() {
		time.Sleep(100 * time.Millisecond)
		n.signal(syscall.SIGTERM, true)
	}()

	n.setStatus(NodeStatusRunning)
	err := n.Execute(context.Background())

	require.Error(t, err)
	require.Equal(t, n.State().Status, NodeStatusCancel)
}

func TestLog(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         "echo",
			Args:            []string{"done"},
			Dir:             os.Getenv("HOME"),
			OutputVariables: &dag.SyncMap{},
		},
	}

	runTestNode(t, n)

	dat, _ := os.ReadFile(n.logFile.Name())
	require.Equal(t, "done\n", string(dat))
}

func TestStdout(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         "echo",
			Args:            []string{"done"},
			Dir:             os.Getenv("HOME"),
			Stdout:          "stdout.log",
			OutputVariables: &dag.SyncMap{},
		},
	}

	runTestNode(t, n)

	f := path.Join(os.Getenv("HOME"), n.step.Stdout)
	dat, _ := os.ReadFile(f)
	require.Equal(t, "done\n", string(dat))
}

func TestStderr(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command: "sh",
			Script: `
echo Stdout message >&1
echo Stderr message >&2
			`,
			Dir:             os.Getenv("HOME"),
			Stdout:          "test-stderr-stdout.log",
			Stderr:          "test-stderr-stderr.log",
			OutputVariables: &dag.SyncMap{},
		},
	}

	runTestNode(t, n)

	f := path.Join(os.Getenv("HOME"), n.step.Stderr)
	dat, _ := os.ReadFile(f)
	require.Equal(t, "Stderr message\n", string(dat))

	f = path.Join(os.Getenv("HOME"), n.step.Stdout)
	dat, _ = os.ReadFile(f)
	require.Equal(t, "Stdout message\n", string(dat))
}

func TestNode(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         "echo",
			Args:            []string{"hello"},
			OutputVariables: &dag.SyncMap{},
		},
	}
	n.incDoneCount()
	require.Equal(t, 1, n.getDoneCount())

	n.incRetryCount()
	require.Equal(t, 1, n.getRetryCount())

	n.id = 1
	n.init()
	require.Nil(t, n.step.Variables)

	n.id = 0
	n.init()
	require.Equal(t, n.step.Variables, []string{})
}

func TestOutput(t *testing.T) {
	n := &Node{
		step: dag.Step{
			CmdWithArgs:     "echo hello",
			Output:          "OUTPUT_TEST",
			OutputVariables: &dag.SyncMap{},
		},
	}
	err := n.setup(os.Getenv("HOME"), "test-request-id-output")
	require.NoError(t, err)
	defer func() {
		_ = n.teardown()
	}()

	runTestNode(t, n)

	dat, _ := os.ReadFile(n.logFile.Name())
	require.Equal(t, "hello\n", string(dat))
	require.Equal(t, "hello", os.ExpandEnv("$OUTPUT_TEST"))

	// Use the previous output in the subsequent step
	n2 := &Node{
		step: dag.Step{
			CmdWithArgs:     "echo $OUTPUT_TEST",
			Output:          "OUTPUT_TEST2",
			OutputVariables: &dag.SyncMap{},
		},
	}

	runTestNode(t, n2)
	require.Equal(t, "hello", os.ExpandEnv("$OUTPUT_TEST2"))

	// Use the previous output in the subsequent step inside a script
	n3 := &Node{
		step: dag.Step{
			Command:         "sh",
			Script:          "echo $OUTPUT_TEST2",
			Output:          "OUTPUT_TEST3",
			OutputVariables: &dag.SyncMap{},
		},
	}

	runTestNode(t, n3)
	require.Equal(t, "hello", os.ExpandEnv("$OUTPUT_TEST3"))
}

func TestOutputJson(t *testing.T) {
	for i, test := range []struct {
		CmdWithArgs string
		Want        string
		WantArgs    int
	}{
		{
			CmdWithArgs: `echo {\"key\":\"value\"}`,
			Want:        `{"key":"value"}`,
			WantArgs:    1,
		},
		{
			CmdWithArgs: `echo "{\"key\": \"value\"}"`,
			Want:        `{"key": "value"}`,
			WantArgs:    1,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			n := &Node{
				step: dag.Step{
					CmdWithArgs:     test.CmdWithArgs,
					Output:          "OUTPUT_JSON_TEST",
					OutputVariables: &dag.SyncMap{},
				},
			}
			err := n.setup(os.Getenv("HOME"), fmt.Sprintf("test-output-jsondb-%d", i))
			require.NoError(t, err)
			defer func() {
				_ = n.teardown()
			}()

			runTestNode(t, n)

			require.Equal(t, test.WantArgs, len(n.step.Args))

			v, _ := n.step.OutputVariables.Load("OUTPUT_JSON_TEST")
			require.Equal(t, fmt.Sprintf("OUTPUT_JSON_TEST=%s", test.Want), v)
			require.Equal(t, test.Want, os.ExpandEnv("$OUTPUT_JSON_TEST"))
		})
	}
}

func TestOutputSpecialchar(t *testing.T) {
	for i, test := range []struct {
		CmdWithArgs string
		Want        string
		WantArgs    int
	}{
		{
			CmdWithArgs: `echo "hello\tworld"`,
			Want:        `hello\tworld`,
			WantArgs:    1,
		},
		{
			CmdWithArgs: `echo hello"\t"world`,
			Want:        `hello\tworld`,
			WantArgs:    1,
		},
		{
			CmdWithArgs: `echo hello\tworld`,
			Want:        `hello\tworld`,
			WantArgs:    1,
		},
		{
			CmdWithArgs: `echo hello\nworld`,
			Want:        `hello\nworld`,
			WantArgs:    1,
		},
		{
			CmdWithArgs: `echo {\"key\":\"value\"}`,
			Want:        `{"key":"value"}`,
			WantArgs:    1,
		},
		{
			CmdWithArgs: `echo "{\"key\":\"value\"}"`,
			Want:        `{"key":"value"}`,
			WantArgs:    1,
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			n := &Node{
				step: dag.Step{
					CmdWithArgs:     test.CmdWithArgs,
					Output:          "OUTPUT_SPECIALCHAR_TEST",
					OutputVariables: &dag.SyncMap{},
				},
			}
			err := n.setup(os.Getenv("HOME"), fmt.Sprintf("test-output-specialchar-%d", i))
			require.NoError(t, err)
			defer func() {
				_ = n.teardown()
			}()

			runTestNode(t, n)

			require.Equal(t, test.WantArgs, len(n.step.Args))

			v, _ := n.step.OutputVariables.Load("OUTPUT_SPECIALCHAR_TEST")
			require.Equal(t, fmt.Sprintf("OUTPUT_SPECIALCHAR_TEST=%s", test.Want), v)
			require.Equal(t, test.Want, os.ExpandEnv("$OUTPUT_SPECIALCHAR_TEST"))
		})
	}
}

func TestRunScript(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command: "sh",
			Args:    []string{},
			Script: `
			  echo hello
			`,
			Output:          "SCRIPT_TEST",
			OutputVariables: &dag.SyncMap{},
		},
	}

	err := n.setup(os.Getenv("HOME"),
		fmt.Sprintf("test-request-id-%d", rand.Int()))
	require.NoError(t, err)

	require.FileExists(t, n.logFile.Name())
	b, _ := os.ReadFile(n.scriptFile.Name())
	require.Equal(t, n.step.Script, string(b))

	require.NoError(t, err)
	err = n.Execute(context.Background())
	require.NoError(t, err)
	err = n.teardown()
	require.NoError(t, err)

	require.Equal(t, "hello", os.Getenv("SCRIPT_TEST"))
	require.NoFileExists(t, n.scriptFile.Name())
}

func TestTeardown(t *testing.T) {
	n := &Node{
		step: dag.Step{
			Command:         testCommand,
			Args:            []string{},
			OutputVariables: &dag.SyncMap{},
		},
	}

	runTestNode(t, n)

	// no error since done flag is true
	err := n.teardown()
	require.NoError(t, err)
	require.NoError(t, n.Error)

	// error
	n.done = false
	err = n.teardown()
	require.Error(t, err)
	require.Error(t, n.Error)
}

func runTestNode(t *testing.T, n *Node) {
	t.Helper()
	err := n.setup(os.Getenv("HOME"),
		fmt.Sprintf("test-request-id-%d", rand.Int()))
	require.NoError(t, err)
	err = n.Execute(context.Background())
	require.NoError(t, err)
	err = n.teardown()
	require.NoError(t, err)
}
