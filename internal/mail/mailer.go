package mail

import (
	"encoding/base64"
	"log"
	"net/smtp"
	"strings"
)

type Mailer interface {
	SendMail(from string, to []string, subject, body string) error
}

type mailer struct {
	*Config
}

type Config struct {
	Host, Port string
}

func New(config *Config) Mailer {
	return &mailer{
		Config: config,
	}
}

func (m *mailer) SendMail(from string, to []string, subject, body string) error {
	log.Printf("Sending an email to %s, subject is \"%s\"", strings.Join(to, ","), subject)
	r := strings.NewReplacer("\r\n", "", "\r", "", "\n", "", "%0a", "", "%0d", "")

	c, err := smtp.Dial(m.Host + ":" + m.Port)
	if err != nil {
		return err
	}
	defer c.Close()
	if err = c.Mail(r.Replace(from)); err != nil {
		return err
	}
	for i := range to {
		to[i] = r.Replace(to[i])
		if err = c.Rcpt(to[i]); err != nil {
			return err
		}
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
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
	err = wc.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}
