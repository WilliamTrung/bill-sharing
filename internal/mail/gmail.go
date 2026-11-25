package mail

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Gmail sends mail via SMTP (e.g. smtp.gmail.com:587 with an app password).
type Gmail struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func (g Gmail) Enabled() bool {
	return strings.TrimSpace(g.User) != "" && g.Pass != ""
}

func (g Gmail) Send(to, subject, body string) error {
	if !g.Enabled() {
		return nil
	}
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("mail: empty recipient")
	}
	from := strings.TrimSpace(g.From)
	if from == "" {
		from = g.User
	}
	host := strings.TrimSpace(g.Host)
	if host == "" {
		host = "smtp.gmail.com"
	}
	port := g.Port
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	auth := smtp.PlainAuth("", g.User, g.Pass, host)
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body,
	))
	return smtp.SendMail(addr, auth, from, []string{to}, msg)
}