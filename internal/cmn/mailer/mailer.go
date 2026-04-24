// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/logger"
	"github.com/dagucloud/dagu/internal/cmn/logger/tag"
)

// Client is a mailer that sends emails.
type Client struct {
	host     string
	port     string
	username string
	password string
}

// Config is a config for SMTP mailer.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
}

func New(cfg Config) *Client {
	return &Client{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
	}
}

var (
	replacer = strings.NewReplacer(
		"\r\n", "", "\r", "", "\n", "", "%0a", "", "%0d", "",
	)
	boundary     = "==simple-boundary-dagu-mailer"
	errFileEmpty = errors.New("file is empty")
	mailTimeout  = 30 * time.Second
)

// SendMail sends an email.
func (m *Client) Send(
	ctx context.Context,
	from string,
	to []string,
	subject, body string,
	attachments []string,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, mailTimeout)
	defer cancel()

	logger.Info(ctx, "Sending email", slog.Any("to", to), tag.Subject(subject))
	if m.username == "" && m.password == "" {
		return m.send(ctx, from, to, subject, body, attachments, false)
	}
	return m.send(ctx, from, to, subject, body, attachments, true)
}

func (m *Client) sendWithNoAuth(
	from string,
	to []string,
	subject, body string,
	attachments []string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), mailTimeout)
	defer cancel()
	return m.send(ctx, from, to, subject, body, attachments, false)
}

func (m *Client) sendWithAuth(
	from string,
	to []string,
	subject, body string,
	attachments []string,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), mailTimeout)
	defer cancel()
	return m.send(ctx, from, to, subject, body, attachments, true)
}

func (m *Client) send(
	ctx context.Context,
	from string,
	to []string,
	subject, body string,
	attachments []string,
	useAuth bool,
) error {
	dialer := &net.Dialer{
		Timeout: mailTimeout,
	}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(m.host, m.port))
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	}

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		return err
	}
	defer func() {
		_ = c.Close()
	}()

	if useAuth {
		if err := m.enableAuth(ctx, c); err != nil {
			return err
		}
	}

	recipients := sanitizeAddresses(to)
	if err := c.Mail(replacer.Replace(from)); err != nil {
		if useAuth {
			return fmt.Errorf("MAIL FROM failed: %w", err)
		}
		return err
	}
	for _, recipient := range recipients {
		if err := c.Rcpt(recipient); err != nil {
			if useAuth {
				return fmt.Errorf("RCPT TO failed: %w", err)
			}
			return err
		}
	}

	wc, err := c.Data()
	if err != nil {
		if useAuth {
			return fmt.Errorf("DATA command failed: %w", err)
		}
		return err
	}
	defer func() {
		_ = wc.Close()
	}()

	payload := m.composeMail(recipients, from, subject, processEmailBody(body), attachments)
	_, err = wc.Write(payload)
	if err != nil {
		if useAuth {
			return fmt.Errorf("failed to write email body: %w", err)
		}
		return err
	}
	if err := wc.Close(); err != nil {
		if useAuth {
			return fmt.Errorf("failed to close data writer: %w", err)
		}
		return err
	}

	return c.Quit()
}

func sanitizeAddresses(addresses []string) []string {
	if len(addresses) == 0 {
		return nil
	}
	cleaned := make([]string, len(addresses))
	for i, address := range addresses {
		cleaned[i] = replacer.Replace(address)
	}
	return cleaned
}

// enableAuth negotiates STARTTLS if available and then authenticates.
func (m *Client) enableAuth(ctx context.Context, c *smtp.Client) error {
	if err := c.Hello("localhost"); err != nil {
		return fmt.Errorf("HELO failed: %w", err)
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: m.host,
			MinVersion: tls.VersionTLS12,
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	}

	if err := m.authenticate(ctx, c); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	return nil
}

// authenticate tries LOGIN auth first, then falls back to PLAIN auth.
// LOGIN auth is more commonly supported for "basic authentication" scenarios.
func (m *Client) authenticate(ctx context.Context, c *smtp.Client) error {
	// Check if server advertises AUTH extension
	if ok, _ := c.Extension("AUTH"); !ok {
		// Server doesn't advertise AUTH - this is unusual for servers requiring auth
		// but we'll let the mail commands fail naturally if auth was actually required
		logger.Debug(ctx, "SMTP server does not advertise AUTH extension",
			slog.String("host", m.host), slog.String("port", m.port))
		return nil
	}

	// Try LOGIN auth first (more widely supported for "basic auth")
	loginAuth := &loginAuth{
		username: m.username,
		password: m.password,
	}
	loginErr := c.Auth(loginAuth)
	if loginErr == nil {
		return nil
	}

	// Fall back to PLAIN auth if LOGIN fails
	plainAuth := smtp.PlainAuth("", m.username, m.password, m.host)
	plainErr := c.Auth(plainAuth)
	if plainErr == nil {
		return nil
	}

	// Both failed - return a combined error message
	return fmt.Errorf("LOGIN auth failed: %v; PLAIN auth failed: %v", loginErr, plainErr)
}

// loginAuth implements smtp.Auth interface for LOGIN authentication mechanism.
// LOGIN auth is different from PLAIN auth - it sends username and password
// in separate base64-encoded exchanges rather than combined.
type loginAuth struct {
	username string
	password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	// LOGIN auth can work over TLS or on localhost
	if !server.TLS {
		// Check for localhost
		if server.Name != "localhost" && server.Name != "127.0.0.1" && server.Name != "::1" {
			return "", nil, errors.New("LOGIN auth requires TLS connection")
		}
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	prompt := strings.ToLower(string(fromServer))
	switch {
	case strings.Contains(prompt, "username"):
		return []byte(a.username), nil
	case strings.Contains(prompt, "password"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected server prompt: %s", fromServer)
	}
}

func (*Client) composeHeader(
	to []string, from string, subject string,
) string {
	return "To: " + strings.Join(to, ",") + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: multipart/mixed;\r\n" +
		"  boundary=\"" + boundary + "\"\r\n\r\n" +
		"\r\n\r\n" +
		"--" + boundary + "\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n"
}

func (m *Client) composeMail(
	to []string,
	from, subject, body string,
	attachments []string,
) []byte {
	var buf bytes.Buffer
	buf.WriteString(m.composeHeader(to, from, subject))
	buf.WriteString("\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(body)))
	buf.Write(addAttachments(attachments))
	buf.WriteString("\r\n\r\n--")
	buf.WriteString(boundary)
	buf.WriteString("--\r\n\r\n")
	buf.WriteString("\r\n\r\n")
	return buf.Bytes()
}

func newlineToBrTag(body string) string {
	return strings.NewReplacer(
		`\r\n`, "<br />", `\r`, "<br />", `\n`, "<br />", "\r\n", "<br />", "\r", "<br />", "\n", "<br />",
	).Replace(body)
}

// isHTMLContent detects if the body content is HTML by checking for DOCTYPE declaration
// This is a restrictive check to ensure we only skip newline conversion for proper HTML documents
func isHTMLContent(body string) bool {
	body = strings.TrimSpace(strings.ToLower(body))
	return strings.HasPrefix(body, "<!doctype html")
}

// processEmailBody converts newlines to <br /> tags for non-HTML (plain text) content.
func processEmailBody(body string) string {
	if !isHTMLContent(body) {
		return newlineToBrTag(body)
	}
	return body
}

func addAttachments(attachments []string) []byte {
	var buf bytes.Buffer
	for _, fileName := range attachments {
		data, err := readFile(fileName)
		if err == nil {
			_, _ = fmt.Fprintf(&buf, "\r\n\n--%s\r\n", boundary)
			_, _ = buf.WriteString("Content-Type: text/plain;" + "\r\n")
			_, _ = buf.WriteString("Content-Transfer-Encoding: base64" + "\r\n")
			_, _ = buf.WriteString(
				"Content-Disposition: attachment; filename=" +
					filepath.Base(fileName) + "\r\n",
			)
			_, _ = buf.WriteString("Content-Transfer-Encoding: base64\r\n\n")
			_, _ = buf.WriteString(base64.StdEncoding.EncodeToString(data))
		}
	}
	return buf.Bytes()
}

func readFile(fileName string) (data []byte, err error) {
	data, err = os.ReadFile(fileName) //nolint:gosec
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errFileEmpty
	}

	return data, nil
}
