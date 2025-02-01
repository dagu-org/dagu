package executor

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/mailer"
	"github.com/go-viper/mapstructure/v2"
)

var _ Executor = (*mail)(nil)

type mail struct {
	stdout io.Writer
	stderr io.Writer
	mailer *mailer.Mailer
	cfg    *mailConfig
}

type mailConfig struct {
	From    string `mapstructure:"from"`
	To      string `mapstructure:"to"`
	Subject string `mapstructure:"subject"`
	Message string `mapstructure:"message"`
}

func newMail(ctx context.Context, step digraph.Step) (Executor, error) {
	var cfg mailConfig
	if err := decodeMailConfig(step.ExecutorConfig.Config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode mail config: %w", err)
	}

	stepContext := digraph.GetStepContext(ctx)

	cfg, err := digraph.EvalStringFields(stepContext, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields: %w", err)
	}

	exec := &mail{cfg: &cfg}
	mailerConfig, err := stepContext.MailerConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields: %w", err)
	}

	exec.mailer = mailer.New(mailerConfig)

	return exec, nil
}

func (e *mail) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *mail) SetStderr(out io.Writer) {
	e.stderr = out
}

func (*mail) Kill(_ os.Signal) error {
	return nil
}

const mailLogTemplate = `sending email
-----
from: %s
to: %s
subject: %s
message: %s
-----
`

func (e *mail) Run(ctx context.Context) error {
	_, _ = e.stdout.Write(
		[]byte(fmt.Sprintf(
			mailLogTemplate,
			e.cfg.From,
			e.cfg.To,
			e.cfg.Subject,
			e.cfg.Message,
		)),
	)
	err := e.mailer.Send(
		ctx,
		e.cfg.From,
		[]string{e.cfg.To},
		e.cfg.Subject,
		e.cfg.Message,
		[]string{},
	)
	if err != nil {
		_, _ = e.stdout.Write([]byte("error occurred."))
	} else {
		_, _ = e.stdout.Write([]byte("sending email succeed."))
	}
	return err
}

func decodeMailConfig(dat map[string]any, cfg *mailConfig) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		Result:           cfg,
	})
	return md.Decode(dat)
}

func init() {
	Register("mail", newMail)
}
