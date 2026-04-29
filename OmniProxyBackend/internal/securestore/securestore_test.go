package securestore

import (
	"strings"
	"testing"
)

func TestProtectStringRoundTrip(t *testing.T) {
	const secret = "sk-test-secret-token"

	protected, err := ProtectString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !IsProtectedString(protected) {
		t.Fatalf("expected protected envelope, got %q", protected)
	}
	if strings.Contains(protected, secret) {
		t.Fatalf("protected value leaked plaintext secret: %q", protected)
	}

	plain, err := UnprotectString(protected)
	if err != nil {
		t.Fatal(err)
	}
	if plain != secret {
		t.Fatalf("expected round trip secret, got %q", plain)
	}
}

func TestUnprotectStringAcceptsLegacyPlaintext(t *testing.T) {
	const secret = "sk-legacy-token"
	plain, err := UnprotectString(secret)
	if err != nil {
		t.Fatal(err)
	}
	if plain != secret {
		t.Fatalf("expected plaintext passthrough, got %q", plain)
	}
}
