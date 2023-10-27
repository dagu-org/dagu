package mailer

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
)

// Mailer is a mailer that sends emails.
type Mailer struct {
	*Config
}

// Config is a config for SMTP mailer.
type Config struct {
	Host     string
	Port     string
	Username string
	Password string
}

var (
	replacer = strings.NewReplacer("\r\n", "", "\r", "", "\n", "", "%0a", "", "%0d", "")
	boundary = "==simple-boundary-dagu-mailer"
)

// SendMail sends an email.
func (m *Mailer) SendMail(from string, to []string, subject, body string, attachments []string) error {
	log.Printf("Sending an email to %s, subject is \"%s\"", strings.Join(to, ","), subject)
	if m.Username == "" && m.Password == "" {
		return m.sendWithNoAuth(from, to, subject, body, attachments)
	}
	return m.sendWithAuth(from, to, subject, body, attachments)
}

func (m *Mailer) sendWithNoAuth(from string, to []string, subject, body string, attachments []string) error {
	c, err := smtp.Dial(m.Host + ":" + m.Port)
	if err != nil {
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
	body = newlineToBrTag(body)
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

func (m *Mailer) sendWithAuth(from string, to []string, subject, body string, attachments []string) error {
	auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
	body = newlineToBrTag(body)
	return smtp.SendMail(
		m.Host+":"+m.Port, auth, from, to,
		m.composeMail(to, from, subject, body, attachments),
	)
}

func (m *Mailer) composeHeader(to []string, from string, subject string) string {
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

func (m *Mailer) composeMail(to []string, from, subject, body string, attachments []string) (b []byte) {
	msg := m.composeHeader(to, from, subject) +
		"\r\n" + base64.StdEncoding.EncodeToString([]byte(body))
	b = joinBytes([]byte(msg), addAttachments(attachments))
	b = joinBytes(b, []byte("\r\n\r\n--"+boundary+"--\r\n\r\n"))
	b = joinBytes(b, []byte("\r\n\r\n"))
	return
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
	return strings.NewReplacer(`\r\n`, "<br />", `\r`, "<br />", `\n`, "<br />").Replace(body)
}

func addAttachments(attachments []string) []byte {
	var buf bytes.Buffer
	for _, fileName := range attachments {
		data, err := readFile(fileName)
		if err == nil {
			buf.WriteString(fmt.Sprintf("\r\n\n--%s\r\n", boundary))
			buf.WriteString("Content-Type: text/plain;" + "\r\n")
			buf.WriteString("Content-Transfer-Encoding: base64" + "\r\n")
			buf.WriteString("Content-Disposition: attachment; filename=" + filepath.Base(fileName) + "\r\n")
			buf.WriteString("Content-Transfer-Encoding: base64\r\n\n")
			b := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
			base64.StdEncoding.Encode(b, data)
			buf.Write(b)
		}
	}
	return buf.Bytes()
}

func readFile(fileName string) (data []byte, err error) {
	data, err = os.ReadFile(fileName)
	if err != nil {
		return nil, err
	} else {
		if len(data) == 0 {
			err = errors.New("file is empty")
		}
	}
	return
}
