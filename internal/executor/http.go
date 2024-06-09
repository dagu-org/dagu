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

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/go-resty/resty/v2"
	"github.com/mitchellh/mapstructure"
)

type HTTPExecutor struct {
	stdout    io.Writer
	req       *resty.Request
	reqCancel context.CancelFunc
	url       string
	method    string
	cfg       *HTTPConfig
}

type HTTPConfig struct {
	Timeout int               `json:"timeout"`
	Headers map[string]string `json:"headers"`
	Query   map[string]string `json:"query"`
	Body    string            `json:"body"`
	Silent  bool              `json:"silent"`
	Debug   bool              `json:"debug"`
}

var errHttpStatusCode = errors.New("http status code not 2xx")

func (e *HTTPExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *HTTPExecutor) SetStderr(out io.Writer) {
	e.stdout = out
}

func (e *HTTPExecutor) Kill(sig os.Signal) error {
	e.reqCancel()
	return nil
}

func (e *HTTPExecutor) Run() error {
	rsp, err := e.req.Execute(strings.ToUpper(e.method), e.url)
	if err != nil {
		return err
	}

	resCode := rsp.StatusCode()
	isErr := resCode < 200 || resCode > 299
	if isErr || !e.cfg.Silent {
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
	if isErr {
		return fmt.Errorf("%w: %d", errHttpStatusCode, resCode)
	}
	return nil
}

func CreateHTTPExecutor(ctx context.Context, step dag.Step) (Executor, error) {
	var reqCfg HTTPConfig
	if len(step.Script) > 0 {
		if err := decodeHTTPConfigFromString(step.Script, &reqCfg); err != nil {
			return nil, err
		}
	} else if step.ExecutorConfig.Config != nil {
		if err := decodeHTTPConfig(step.ExecutorConfig.Config, &reqCfg); err != nil {
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

	return &HTTPExecutor{
		stdout:    os.Stdout,
		req:       req,
		reqCancel: cancel,
		method:    step.Command,
		url:       step.Args[0],
		cfg:       &reqCfg,
	}, nil
}

func decodeHTTPConfig(dat map[string]interface{}, cfg *HTTPConfig) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: false,
		Result:      cfg,
	})
	return md.Decode(dat)
}

func decodeHTTPConfigFromString(s string, cfg *HTTPConfig) error {
	if len(s) > 0 {
		ss := os.ExpandEnv(s)
		if err := json.Unmarshal([]byte(ss), &cfg); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	Register("http", CreateHTTPExecutor)
}
