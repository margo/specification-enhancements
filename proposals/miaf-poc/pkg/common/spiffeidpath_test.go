package common_test

import (
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
)

func TestEncodeSPIFFEID_SUPExample(t *testing.T) {
	// Exact example from SUP §"Common URI and Encoding Rules".
	const in = "spiffe://northstar-ida.com/margo/device/123e4567-e89b-12d3-a456-426614174000"
	const want = "c3BpZmZlOi8vbm9ydGhzdGFyLWlkYS5jb20vbWFyZ28vZGV2aWNlLzEyM2U0NTY3LWU4OWItMTJkMy1hNDU2LTQyNjYxNDE3NDAwMA"
	if got := common.EncodeSPIFFEID(in); got != want {
		t.Errorf("EncodeSPIFFEID = %q\nwant                %q", got, want)
	}
}

func TestEncodeSPIFFEID_NoPadding(t *testing.T) {
	for _, in := range []string{
		"spiffe://td/a",
		"spiffe://td/ab",
		"spiffe://td/abc",
	} {
		got := common.EncodeSPIFFEID(in)
		for _, r := range got {
			if r == '=' {
				t.Errorf("EncodeSPIFFEID(%q) contains padding: %q", in, got)
			}
		}
	}
}
