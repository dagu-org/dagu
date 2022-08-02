package executor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/yohamta/dagu/internal/config"
)

type HTTPExecutor struct {
	stdout    io.Writer
	req       *resty.Request
	reqCancel context.CancelFunc
	url       string
	method    string
}

type HTTPConfig struct {
	Timeout     int               `json:"timeout"`
	Headers     map[string]string `json:"headers"`
	QueryParams map[string]string `json:"query"`
	Body        string            `json:"body"`
}

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
	if _, err := e.stdout.Write([]byte(rsp.Status() + "\n")); err != nil {
		return err
	}
	if err := rsp.Header().Write(e.stdout); err != nil {
		return err
	}
	if _, err := e.stdout.Write(rsp.Body()); err != nil {
		return err
	}
	if rsp.StatusCode() != 200 {
		return errors.New("http status code not 200")
	}
	return nil
}

func CreateHTTPExecutor(ctx context.Context, step *config.Step) (Executor, error) {
	var reqCfg HTTPConfig
	if len(step.Script) > 0 {
		if err := json.Unmarshal([]byte(step.Script), &reqCfg); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	client := resty.New()
	if reqCfg.Timeout > 0 {
		client.SetTimeout(time.Duration(reqCfg.Timeout))
	}
	req := client.R().SetContext(ctx)
	if len(reqCfg.Headers) > 0 {
		req = req.SetHeaders(reqCfg.Headers)
	}
	if len(reqCfg.QueryParams) > 0 {
		req = req.SetQueryParams(reqCfg.QueryParams)
	}
	req = req.SetBody([]byte(reqCfg.Body))

	return &HTTPExecutor{
		stdout:    os.Stdout,
		req:       req,
		reqCancel: cancel,
		method:    step.Command,
		url:       step.Args[0],
	}, nil
}

func init() {
	Register("http", CreateHTTPExecutor)
}
