package mailer

import (
	"encoding/base64"
	"log"
	"net/smtp"
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
)

// SendMail sends an email.
func (m *Mailer) SendMail(from string, to []string, subject, body string) error {
	log.Printf("Sending an email to %s, subject is \"%s\"", strings.Join(to, ","), subject)
	if m.Username == "" && m.Password == "" {
		return m.sendWithNoAuth(from, to, subject, body)
	}
	return m.sendWithAuth(from, to, subject, body)
}

func (m *Mailer) sendWithNoAuth(from string, to []string, subject, body string) error {
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
	msg := "To: " + strings.Join(to, ",") + "\r\n" +
		"From: " + from + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"\r\n" + base64.StdEncoding.EncodeToString([]byte(body))
	_, err = wc.Write([]byte(msg))
	if err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func (m *Mailer) sendWithAuth(from string, to []string, subject, body string) error {
	auth := smtp.PlainAuth("", m.Username, m.Password, m.Host)
	body = newlineToBrTag(body)
	return smtp.SendMail(m.Host+":"+m.Port, auth, from, to, []byte("To: "+strings.Join(to, ",")+"\r\n"+
		"From: "+from+"\r\n"+
		"Subject: "+subject+"\r\n"+
		"Content-Type: text/html; charset=\"UTF-8\"\r\n"+
		"Content-Transfer-Encoding: base64\r\n"+
		"\r\n"+base64.StdEncoding.EncodeToString([]byte(body))))
}

func newlineToBrTag(body string) string {
	return strings.NewReplacer(`\r\n`, "<br />", `\r`, "<br />", `\n`, "<br />").Replace(body)
}
