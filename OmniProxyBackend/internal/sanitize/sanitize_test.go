package sanitize

import (
	"strings"
	"testing"
)

func TestTextRedactsSecrets(t *testing.T) {
	input := "Authorization: Bearer sk-secret-token-1234567890&api_key=sk-query-secret&access_token=abc123456789"
	out := Text(input)
	for _, secret := range []string{"sk-secret-token-1234567890", "sk-query-secret", "abc123456789"} {
		if strings.Contains(out, secret) {
			t.Fatalf("expected %q to be redacted from %q", secret, out)
		}
	}
	for _, expected := range []string{"Authorization: Bearer ***", "api_key=***", "access_token=***"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected %q in redacted output %q", expected, out)
		}
	}
}
