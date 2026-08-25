package common_test

import (
	"encoding/json"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
)

func TestJWTSVIDExchangeRequestDTO_RoundTrip(t *testing.T) {
	in := common.JWTSVIDExchangeRequestDTO{
		Audience:        []string{"https://dfm.example/"},
		TTLSeconds:      300,
		ClientAssertion: "eyJ...",
	}
	b, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var out common.JWTSVIDExchangeRequestDTO
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Audience[0] != "https://dfm.example/" || out.TTLSeconds != 300 || out.ClientAssertion != "eyJ..." {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

func TestJWTSVIDExchangeRequestDTO_EmptyOptionalFields(t *testing.T) {
	in := common.JWTSVIDExchangeRequestDTO{Audience: []string{"https://x/"}}
	b, _ := json.Marshal(&in)
	if string(b) != `{"aud":["https://x/"]}` {
		t.Errorf("encoded = %s, want minimal", b)
	}
}
