package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"
	"github.com/yohamta/dagu/internal/dag"
)

type JqExecutor struct {
	stdout io.Writer
	stderr io.Writer
	query  string
	input  map[string]interface{}
}

func (e *JqExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *JqExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *JqExecutor) Kill(sig os.Signal) error {
	return nil
}

func (e *JqExecutor) Run() error {
	query, err := gojq.Parse(e.query)
	if err != nil {
		return err
	}
	iter := query.Run(e.input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			e.stderr.Write([]byte(fmt.Sprintf("%#v", err)))
			continue
		}
		val, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			e.stderr.Write([]byte(fmt.Sprintf("%#v", err)))
			continue
		}
		e.stdout.Write(val)
	}
	return nil
}

func CreateJqExecutor(ctx context.Context, step *dag.Step) (Executor, error) {
	s := os.ExpandEnv(step.Script)
	input := map[string]interface{}{}
	if err := json.Unmarshal([]byte(s), &input); err != nil {
		return nil, err
	}
	return &JqExecutor{
		stdout: os.Stdout,
		input:  input,
		query:  step.CmdWithArgs,
	}, nil
}

func init() {
	Register("jq", CreateJqExecutor)
}
