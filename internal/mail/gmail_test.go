package mail

import (
	"strings"
	"testing"
)

func TestAppPasswordStripsSpaces(t *testing.T) {
	got := appPassword("pemn nqsx bmcw njyu")
	if got != "pemnnqsxbmcwnjyu" {
		t.Fatalf("appPassword = %q", got)
	}
	if appPassword("  already-compact  ") != "already-compact" {
		t.Fatalf("appPassword did not trim: %q", appPassword("  already-compact  "))
	}
}

func TestFormatAddressUsesDisplayName(t *testing.T) {
	got := formatAddress("Share da Bill", "bills@example.com")
	if got != "Share da Bill <bills@example.com>" {
		t.Fatalf("formatAddress = %q", got)
	}
}

func TestFormatAddressEncodesNonASCII(t *testing.T) {
	got := formatAddress("Chia Hoá Đơn", "bills@example.com")
	if !strings.HasPrefix(got, "=?UTF-8?B?") || !strings.HasSuffix(got, "?= <bills@example.com>") {
		t.Fatalf("non-ASCII name not encoded: %q", got)
	}
}

func TestFormatAddressWithoutName(t *testing.T) {
	if got := formatAddress("   ", "bills@example.com"); got != "bills@example.com" {
		t.Fatalf("formatAddress = %q", got)
	}
}

func TestBuildMessageHeaders(t *testing.T) {
	msg := string(buildMessage("Share da Bill", "bills@example.com", "sam@example.com", "Dinner", "Hello"))
	if !strings.Contains(msg, "From: Share da Bill <bills@example.com>\r\n") {
		t.Fatalf("missing display-name From header:\n%s", msg)
	}
	if !strings.Contains(msg, "To: sam@example.com\r\n") {
		t.Fatalf("missing To header:\n%s", msg)
	}
	if !strings.Contains(msg, "Subject: Dinner\r\n") {
		t.Fatalf("missing Subject header:\n%s", msg)
	}
	if !strings.HasSuffix(msg, "\r\n\r\nHello") {
		t.Fatalf("body not appended after blank line:\n%s", msg)
	}
}

func TestBuildMessageEncodesNonASCIISubject(t *testing.T) {
	msg := string(buildMessage("Share da Bill", "bills@example.com", "sam@example.com", "Hoá đơn mới", "Hello"))
	if !strings.Contains(msg, "Subject: =?UTF-8?B?") {
		t.Fatalf("non-ASCII subject not encoded:\n%s", msg)
	}
}
