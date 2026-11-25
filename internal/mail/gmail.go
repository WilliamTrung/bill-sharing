package mail

import (
	"encoding/base64"
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
	Name string
}

const DefaultName = "Share da Bill"

func (g Gmail) Enabled() bool {
	return strings.TrimSpace(g.User) != "" && strings.TrimSpace(g.Pass) != ""
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
		from = strings.TrimSpace(g.User)
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
	auth := smtp.PlainAuth("", strings.TrimSpace(g.User), appPassword(g.Pass), host)
	return smtp.SendMail(addr, auth, from, []string{to}, buildMessage(g.Name, from, to, subject, body))
}

// appPassword strips the grouping spaces Gmail shows in app passwords.
func appPassword(pass string) string {
	return strings.ReplaceAll(strings.TrimSpace(pass), " ", "")
}

func buildMessage(name, from, to, subject, body string) []byte {
	return []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		formatAddress(name, from), to, encodeHeader(subject), body,
	))
}

// formatAddress renders `Name <addr>`, encoding the name when it is not plain ASCII.
func formatAddress(name, addr string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return addr
	}
	return fmt.Sprintf("%s <%s>", encodeHeader(name), addr)
}

// encodeHeader emits RFC 2047 encoded-words for non-ASCII text, else the raw value.
func encodeHeader(value string) string {
	if isASCII(value) {
		return value
	}
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(value)) + "?="
}

func isASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	return true
}
