package email

import (
	"strings"
	"testing"
)

func TestSanitizeHeader(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"hello", "hello"},
		{"hi\r\nBcc: attacker@evil.com", "hiBcc: attacker@evil.com"},
		{"a\nb", "ab"},
		{"a\rb", "ab"},
		{"line1\r\nline2\nline3", "line1line2line3"},
	}
	for _, c := range cases {
		got := SanitizeHeader(c.in)
		if got != c.want {
			t.Errorf("SanitizeHeader(%q) = %q, want %q", c.in, got, c.want)
		}
		if strings.ContainsAny(got, "\r\n") {
			t.Errorf("SanitizeHeader(%q) содержит CR/LF: %q", c.in, got)
		}
	}
}

func TestSanitizeHeaderTruncates(t *testing.T) {
	long := strings.Repeat("a", 2000)
	got := SanitizeHeader(long)
	if len(got) != 1000 {
		t.Errorf("должно быть обрезано до 1000, got %d", len(got))
	}
}
