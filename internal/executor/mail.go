package executor

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/mailer"
)

type MailExecutor struct {
	stdout  io.Writer
	stderr  io.Writer
	from    string
	to      string
	subject string
	message string
	mailer  *mailer.Mailer
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

func (e *MailExecutor) Run() error {
	return e.mailer.SendMail(e.from, []string{e.to}, e.subject, e.message)
}

func CreateMailExecutor(ctx context.Context, step *dag.Step) (Executor, error) {
	exec := &MailExecutor{}

	for _, key := range []string{"from", "to", "subject", "message"} {
		if _, ok := step.ExecutorConfig.Config[key]; !ok {
			return nil, fmt.Errorf(`"%s" is required`, key)
		}
		switch v := step.ExecutorConfig.Config[key].(type) {
		case string:
			switch key {
			case "from":
				exec.from = os.ExpandEnv(v)
			case "to":
				exec.to = os.ExpandEnv(v)
			case "subject":
				exec.subject = os.ExpandEnv(v)
			case "message":
				exec.message = os.ExpandEnv(v)
			}
		default:
			return nil, fmt.Errorf(`"%s" is must be string`, key)
		}
	}

	d := dag.GetDAGFromContext(ctx)
	m := &mailer.Mailer{
		Config: &mailer.Config{
			Host:     d.Smtp.Host,
			Port:     d.Smtp.Port,
			Username: d.Smtp.Username,
			Password: d.Smtp.Password,
		}}
	exec.mailer = m

	return exec, nil
}

func init() {
	Register("mail", CreateMailExecutor)
}
