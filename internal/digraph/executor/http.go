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

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/go-resty/resty/v2"
	"github.com/go-viper/mapstructure/v2"
)

var _ Executor = (*http)(nil)

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

type httpJSONResult struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       any                 `json:"body"`
}

func newHTTP(ctx context.Context, step digraph.Step) (Executor, error) {
	stepContext := digraph.GetStepContext(ctx)
	var reqCfg httpConfig
	if len(step.Script) > 0 {
		if err := decodeHTTPConfigFromString(ctx, step.Script, &reqCfg); err != nil {
			return nil, err
		}
	} else if step.ExecutorConfig.Config != nil {
		if err := decodeHTTPConfig(
			step.ExecutorConfig.Config, &reqCfg,
		); err != nil {
			return nil, err
		}
		body, err := stepContext.EvalString(reqCfg.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate body: %w", err)
		}
		reqCfg.Body = body
		for k, v := range reqCfg.Headers {
			header, err := stepContext.EvalString(v)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate header %q: %w", k, err)
			}
			reqCfg.Headers[k] = header
		}
		for k, v := range reqCfg.Query {
			query, err := stepContext.EvalString(v)
			if err != nil {
				return nil, fmt.Errorf("failed to evaluate query %q: %w", k, err)
			}
			reqCfg.Query[k] = query
		}
	}

	url, err := stepContext.EvalString(step.Args[0])
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate url: %w", err)
	}

	method, err := stepContext.EvalString(step.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate method: %w", err)
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
		method:    method,
		url:       url,
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
		httpJSONResultData  = &httpJSONResult{}
		err                 error
		httpJSONResultBytes []byte
	)

	if !rsp.IsSuccess() || !e.cfg.Silent {
		httpJSONResultData.Headers = rsp.Header()
		httpJSONResultData.StatusCode = rsp.StatusCode()
	}

	if err = json.Unmarshal(rsp.Body(), &httpJSONResultData.Body); err != nil {
		return err
	}

	if httpJSONResultBytes, err = json.MarshalIndent(httpJSONResultData, "", " "); err != nil {
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

func (e *http) Run(_ context.Context) error {
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
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		Result:           cfg,
	})
	return md.Decode(dat)
}

func decodeHTTPConfigFromString(ctx context.Context, s string, cfg *httpConfig) error {
	stepContext := digraph.GetStepContext(ctx)
	if len(s) > 0 {
		configString, err := stepContext.EvalString(s)
		if err != nil {
			return fmt.Errorf("failed to evaluate http config: %w", err)
		}
		if err := json.Unmarshal([]byte(configString), &cfg); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	Register("http", newHTTP)
}
