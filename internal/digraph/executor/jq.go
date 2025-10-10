package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/go-viper/mapstructure/v2"
	"github.com/itchyny/gojq"
)

var _ digraph.Executor = (*jq)(nil)

type jq struct {
	stdout io.Writer
	stderr io.Writer
	query  string
	input  map[string]any
	cfg    *jqConfig
}

type jqConfig struct {
	Raw bool `mapstructure:"raw"`
}

func newJQ(_ context.Context, step digraph.Step) (digraph.Executor, error) {
	var jqCfg jqConfig
	if step.ExecutorConfig.Config != nil {
		if err := decodeJqConfig(
			step.ExecutorConfig.Config, &jqCfg,
		); err != nil {
			return nil, err
		}
	}
	input := map[string]any{}
	if err := json.Unmarshal([]byte(step.Script), &input); err != nil {
		return nil, err
	}
	return &jq{
		stdout: os.Stdout,
		input:  input,
		query:  step.CmdWithArgs,
		cfg:    &jqCfg,
	}, nil
}

func (e *jq) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *jq) SetStderr(out io.Writer) {
	e.stderr = out
}

func (*jq) Kill(_ os.Signal) error {
	return nil
}

func (e *jq) Run(_ context.Context) error {
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
			_, _ = fmt.Fprintf(e.stderr, "failed to run jq query: %v", err)
			continue
		}
		if e.cfg.Raw {
			// In raw mode, handle strings specially to preserve tabs, newlines, etc
			switch v := v.(type) {
			case string:
				_, _ = fmt.Fprintln(e.stdout, v)
			default:
				// For non-strings, convert to string representation
				val, err := json.Marshal(v)
				if err != nil {
					_, _ = fmt.Fprintf(e.stderr, "failed to marshal jq output: %v", err)
					continue
				}
				_, _ = fmt.Fprintln(e.stdout, strings.Trim(string(val), `"`))
			}
		} else {
			// In non-raw mode, use JSON formatting
			val, err := json.MarshalIndent(v, "", "    ")
			if err != nil {
				_, _ = fmt.Fprintf(e.stderr, "failed to marshal jq output: %v", err)
				continue
			}
			_, _ = e.stdout.Write(val)
			_, _ = e.stdout.Write([]byte("\n"))
		}
	}
	return nil
}

func decodeJqConfig(dat map[string]any, cfg *jqConfig) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		Result:           cfg,
	})
	return md.Decode(dat)
}

func init() {
	digraph.RegisterExecutor("jq", newJQ)
}
