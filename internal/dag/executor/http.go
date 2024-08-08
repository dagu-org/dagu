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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
)

type http struct {
	stdout    io.Writer
	req       *resty.Request
	reqCancel context.CancelFunc
	url       string
	method    string
	cfg       *httpConfig
}

type httpConfig struct {
	Timeout int               `json:"timeout"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    string            `json:"body"`
	Silent  bool              `json:"silent"`
	Debug   bool              `json:"debug"`
	Json    bool              `json:"json"`
}

type HttpJSONResult struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       map[string]any      `json:"body"`
}

func newHTTP(ctx context.Context, step dag.Step) (Executor, error) {
	var reqCfg httpConfig
	if len(step.Script) > 0 {
		if err := decodeHTTPConfigFromString(step.Script, &reqCfg); err != nil {
			return nil, err
		}
	} else if step.ExecutorConfig.Config != nil {
		if err := decodeHTTPConfig(
			step.ExecutorConfig.Config, &reqCfg,
		); err != nil {
			return nil, err
		}
		reqCfg.Body = os.ExpandEnv(reqCfg.Body)
		for k, v := range reqCfg.Headers {
			reqCfg.Headers[k] = os.ExpandEnv(v)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	client := resty.New()
	if reqCfg.Debug {
		client.SetDebug(true)
	}
	if reqCfg.Timeout > 0 {
		client.SetTimeout(time.Second * time.Duration(reqCfg.Timeout))
	}
	req := client.R().SetContext(ctx)
	if len(reqCfg.Headers) > 0 {
		req = req.SetHeaders(reqCfg.Headers)
	}
	if len(reqCfg.Query) > 0 {
		req = req.SetQueryParams(reqCfg.Query)
	}
	req = req.SetBody([]byte(reqCfg.Body))

	return &http{
		stdout:    os.Stdout,
		req:       req,
		reqCancel: cancel,
		method:    step.Command,
		url:       step.Args[0],
		cfg:       &reqCfg,
	}, nil
}

func (e *http) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *http) SetStderr(out io.Writer) {
	e.stdout = out
}

func (e *http) Kill(_ os.Signal) error {
	e.reqCancel()
	return nil
}

var errHTTPStatusCode = errors.New("http status code not 2xx")

func (e *http) writeJSONResult(rsp *resty.Response) error {
	var (
		httpJSONResult      = &HttpJSONResult{}
		err                 error
		httpJSONResultBytes []byte
	)

	if !rsp.IsSuccess() || !e.cfg.Silent {
		httpJSONResult.Headers = rsp.Header()
		httpJSONResult.StatusCode = rsp.StatusCode()
	}

	if err = json.Unmarshal(rsp.Body(), &httpJSONResult.Body); err != nil {
		return err
	}

	if httpJSONResultBytes, err = json.MarshalIndent(httpJSONResult, "", " "); err != nil {
		return err
	}

	if _, err = e.stdout.Write(httpJSONResultBytes); err != nil {
		return err
	}

	return nil
}

func (e *http) writeTextResult(rsp *resty.Response) error {
	if !rsp.IsSuccess() || !e.cfg.Silent {
		if _, err := e.stdout.Write([]byte(rsp.Status() + "\n")); err != nil {
			return err
		}
		if err := rsp.Header().Write(e.stdout); err != nil {
			return err
		}
	}

	if _, err := e.stdout.Write(rsp.Body()); err != nil {
		return err
	}

	return nil
}

func (e *http) Run() error {
	rsp, err := e.req.Execute(strings.ToUpper(e.method), e.url)
	if err != nil {
		return err
	}

	resCode := rsp.StatusCode()

	if e.cfg.Json {
		if err = e.writeJSONResult(rsp); err != nil {
			return err
		}
	} else {
		if err = e.writeTextResult(rsp); err != nil {
			return err
		}
	}

	if !rsp.IsSuccess() {
		return fmt.Errorf("%w: %d", errHTTPStatusCode, resCode)
	}
	return nil
}

func decodeHTTPConfig(dat map[string]any, cfg *httpConfig) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: false,
		Result:      cfg,
	})
	return md.Decode(dat)
}

func decodeHTTPConfigFromString(s string, cfg *httpConfig) error {
	if len(s) > 0 {
		ss := os.ExpandEnv(s)
		if err := json.Unmarshal([]byte(ss), &cfg); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	Register("http", newHTTP)
}
