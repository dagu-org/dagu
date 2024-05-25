package executor

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/mailer"
	"github.com/mitchellh/mapstructure"
)

type MailExecutor struct {
	stdout io.Writer
	stderr io.Writer
	mailer *mailer.Mailer
	cfg    *MailConfig
}

type MailConfig struct {
	From    string `mapstructure:"from"`
	To      string `mapstructure:"to"`
	Subject string `mapstructure:"subject"`
	Message string `mapstructure:"message"`
}

func (e *MailExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *MailExecutor) SetStderr(out io.Writer) {
	e.stderr = out
}

func (e *MailExecutor) Kill(sig os.Signal) error {
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

func (e *MailExecutor) Run() error {
	_, _ = e.stdout.Write([]byte(fmt.Sprintf(mailLogTemplate, e.cfg.From, e.cfg.To, e.cfg.Subject, e.cfg.Message)))
	err := e.mailer.SendMail(e.cfg.From, []string{e.cfg.To}, e.cfg.Subject, e.cfg.Message, []string{})
	if err != nil {
		_, _ = e.stdout.Write([]byte("error occurred."))
	} else {
		_, _ = e.stdout.Write([]byte("sending email succeed."))
	}
	return err
}

func CreateMailExecutor(ctx context.Context, step dag.Step) (Executor, error) {
	var cfg MailConfig
	if err := decodeMailConfig(step.ExecutorConfig.Config, &cfg); err != nil {
		return nil, err
	}

	cfg.From = os.ExpandEnv(cfg.From)
	cfg.To = os.ExpandEnv(cfg.To)
	cfg.Subject = os.ExpandEnv(cfg.Subject)
	cfg.Message = os.ExpandEnv(cfg.Message)

	exec := &MailExecutor{cfg: &cfg}

	dagCtx, err := dag.GetContext(ctx)
	if err != nil {
		return nil, err
	}
	m := &mailer.Mailer{
		Config: &mailer.Config{
			Host:     dagCtx.DAG.Smtp.Host,
			Port:     dagCtx.DAG.Smtp.Port,
			Username: dagCtx.DAG.Smtp.Username,
			Password: dagCtx.DAG.Smtp.Password,
		}}
	exec.mailer = m

	return exec, nil
}

func decodeMailConfig(dat map[string]interface{}, cfg *MailConfig) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: false,
		Result:      cfg,
	})
	return md.Decode(dat)
}

func init() {
	Register("mail", CreateMailExecutor)
}
