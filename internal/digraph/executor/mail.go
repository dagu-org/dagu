// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package executor

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/mailer"
	"github.com/mitchellh/mapstructure"
)

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

	cfg, err := cmdutil.SubstituteStringFields(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields: %w", err)
	}

	exec := &mail{cfg: &cfg}
	dagCtx := digraph.GetContext(ctx)
	mailerConfig, err := dagCtx.MailerConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields: %w", err)
	}

	m := mailer.New(mailerConfig)
	exec.mailer = m

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
		ErrorUnused: false,
		Result:      cfg,
	})
	return md.Decode(dat)
}

func init() {
	Register("mail", newMail)
}
