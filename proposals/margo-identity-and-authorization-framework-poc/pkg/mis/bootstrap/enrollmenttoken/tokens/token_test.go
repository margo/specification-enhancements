package tokens_test

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
)

func TestNewSecret_ShapeAndEntropy(t *testing.T) {
	s, err := tokens.NewSecret(rand.Reader)
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	if !strings.HasPrefix(s.Wire(), tokens.WirePrefix) {
		t.Errorf("wire = %q, want prefix %q", s.Wire(), tokens.WirePrefix)
	}
	if len(s.Bytes()) != tokens.SecretLenBytes {
		t.Errorf("len(bytes) = %d, want %d", len(s.Bytes()), tokens.SecretLenBytes)
	}
}

func TestParseSecret_RoundTrip(t *testing.T) {
	orig, err := tokens.NewSecret(rand.Reader)
	if err != nil {
		t.Fatalf("NewSecret: %v", err)
	}
	parsed, err := tokens.ParseSecret(orig.Wire())
	if err != nil {
		t.Fatalf("ParseSecret: %v", err)
	}
	if !bytes.Equal(parsed.Bytes(), orig.Bytes()) {
		t.Errorf("round-trip bytes mismatch")
	}
	if parsed.Wire() != orig.Wire() {
		t.Errorf("round-trip wire mismatch")
	}
}

func TestParseSecret_Rejects(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"missing-prefix", "not-a-margo-token"},
		{"bad-base64", tokens.WirePrefix + "!!!not-base64!!!"},
		{"wrong-length", tokens.WirePrefix + "QUE"}, // base64url of 2 bytes
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := tokens.ParseSecret(c.in); err == nil {
				t.Errorf("ParseSecret(%q) = nil, want error", c.in)
			}
		})
	}
}

func TestSecretHash_Stable(t *testing.T) {
	s, _ := tokens.NewSecret(rand.Reader)
	if !bytes.Equal(s.Hash(), s.Hash()) {
		t.Error("Hash must be stable")
	}
	if len(s.Hash()) != 32 {
		t.Errorf("len(hash) = %d, want 32", len(s.Hash()))
	}
}

func TestNewTokenID_UUIDv4(t *testing.T) {
	id := tokens.NewTokenID()
	if len(id) != 36 {
		t.Errorf("token_id len = %d, want 36 (UUIDv4 dash-form)", len(id))
	}
	// Two successive calls differ.
	if tokens.NewTokenID() == id {
		t.Error("NewTokenID returned the same id twice")
	}
}
