package http_test

import (
	"errors"
	"testing"

	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestDecodeSPIFFEIDEncoded_SUPExample(t *testing.T) {
	const enc = "c3BpZmZlOi8vbm9ydGhzdGFyLWlkYS5jb20vbWFyZ28vZGV2aWNlLzEyM2U0NTY3LWU4OWItMTJkMy1hNDU2LTQyNjYxNDE3NDAwMA"
	got, err := mishttp.DecodeSPIFFEIDEncoded(enc)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	want := "spiffe://northstar-ida.com/margo/device/123e4567-e89b-12d3-a456-426614174000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDecodeSPIFFEIDEncoded_Rejects(t *testing.T) {
	for _, c := range []struct {
		name, in string
	}{
		{"empty", ""},
		{"padding-forbidden", "c3BpZmZlOi8vdGQvYQ=="},
		{"non-base64url", "spiffe://td/a!"},
		{"not-spiffe-uri", "aHR0cHM6Ly9leGFtcGxlLmNvbS9m"}, // "https://example.com/f"
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := mishttp.DecodeSPIFFEIDEncoded(c.in)
			if !errors.Is(err, mishttp.ErrSPIFFEIDEncoding) {
				t.Errorf("err = %v, want ErrSPIFFEIDEncoding", err)
			}
		})
	}
}
