package mailer

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
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
	logger.Info(ctx, "Sending an email", "to", to, "subject", subject)
	if m.username == "" && m.password == "" {
		return m.sendWithNoAuth(from, to, subject, body, attachments)
	}
	return m.sendWithAuth(from, to, subject, body, attachments)
}

func (m *Client) sendWithNoAuth(
	from string,
	to []string,
	subject, body string,
	attachments []string,
) error {
	// Create a dialer with timeout
	dialer := &net.Dialer{
		Timeout: mailTimeout,
	}

	// Dial with timeout
	conn, err := dialer.Dial("tcp", m.host+":"+m.port)
	if err != nil {
		return err
	}

	// Set deadline for all operations
	deadline := time.Now().Add(mailTimeout)
	if err := conn.SetDeadline(deadline); err != nil {
		_ = conn.Close()
		return err
	}

	c, err := smtp.NewClient(conn, m.host)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer func() {
		_ = c.Close()
	}()

	if err = c.Mail(replacer.Replace(from)); err != nil {
		return err
	}
	for i := range to {
		to[i] = replacer.Replace(to[i])
		if err = c.Rcpt(to[i]); err != nil {
			return err
		}
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
	body = processEmailBody(body)
	_, err = wc.Write(
		m.composeMail(to, from, subject, body, attachments),
	)
	if err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func (m *Client) sendWithAuth(
	from string,
	to []string,
	subject, body string,
	attachments []string,
) error {
	// Create a channel to receive the result
	type result struct {
		err error
	}
	resultChan := make(chan result, 1)

	// Run SendMail in a goroutine
	go func() {
		auth := smtp.PlainAuth("", m.username, m.password, m.host)
		body = processEmailBody(body)
		err := smtp.SendMail(
			m.host+":"+m.port, auth, from, to,
			m.composeMail(to, from, subject, body, attachments),
		)
		resultChan <- result{err: err}
	}()

	// Wait for either completion or timeout
	select {
	case res := <-resultChan:
		return res.err
	case <-time.After(mailTimeout):
		return fmt.Errorf("mail sending timeout after %v", mailTimeout)
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
) (b []byte) {
	msg := m.composeHeader(to, from, subject) +
		"\r\n" + base64.StdEncoding.EncodeToString([]byte(body))
	b = joinBytes([]byte(msg), addAttachments(attachments))
	b = joinBytes(b, []byte("\r\n\r\n--"+boundary+"--\r\n\r\n"))
	b = joinBytes(b, []byte("\r\n\r\n"))
	return b
}

func joinBytes(s ...[]byte) []byte {
	n := 0
	for _, v := range s {
		n += len(v)
	}

	b, i := make([]byte, n), 0
	for _, v := range s {
		i += copy(b[i:], v)
	}
	return b
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
			_, _ = buf.WriteString(fmt.Sprintf("\r\n\n--%s\r\n", boundary))
			_, _ = buf.WriteString("Content-Type: text/plain;" + "\r\n")
			_, _ = buf.WriteString("Content-Transfer-Encoding: base64" + "\r\n")
			_, _ = buf.WriteString(
				"Content-Disposition: attachment; filename=" +
					filepath.Base(fileName) + "\r\n",
			)
			_, _ = buf.WriteString("Content-Transfer-Encoding: base64\r\n\n")
			b := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
			base64.StdEncoding.Encode(b, data)
			_, _ = buf.Write(b)
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
