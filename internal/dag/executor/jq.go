// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/itchyny/gojq"
	"github.com/mitchellh/mapstructure"
)

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

func newJQ(_ context.Context, step dag.Step) (Executor, error) {
	var jqCfg jqConfig
	if step.ExecutorConfig.Config != nil {
		if err := decodeJqConfig(
			step.ExecutorConfig.Config, &jqCfg,
		); err != nil {
			return nil, err
		}
	}
	s := os.ExpandEnv(step.Script)
	input := map[string]any{}
	if err := json.Unmarshal([]byte(s), &input); err != nil {
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

func (e *jq) Run() error {
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
			_, _ = e.stderr.Write([]byte(fmt.Sprintf("%#v", err)))
			continue
		}
		val, err := json.MarshalIndent(v, "", "    ")
		if err != nil {
			_, _ = e.stderr.Write([]byte(fmt.Sprintf("%#v", err)))
			continue
		}
		if e.cfg.Raw {
			s := string(val)
			_, _ = e.stdout.Write([]byte(strings.Trim(s, `"`)))
		} else {
			_, _ = e.stdout.Write(val)
		}
	}
	return nil
}

func decodeJqConfig(dat map[string]any, cfg *jqConfig) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: false,
		Result:      cfg,
	})
	return md.Decode(dat)
}

func init() {
	Register("jq", newJQ)
}
