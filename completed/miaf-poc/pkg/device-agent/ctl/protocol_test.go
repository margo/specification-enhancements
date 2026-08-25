package ctl

import (
	"encoding/json"
	"testing"
)

func TestRequest_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	req := Request{
		Cmd:        "exchange-jwt",
		Audiences:  []string{"https://api.example.com"},
		TTLSeconds: 300,
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Request
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Cmd != req.Cmd || got.TTLSeconds != req.TTLSeconds || len(got.Audiences) != 1 {
		t.Errorf("round-trip mismatch: %+v vs %+v", got, req)
	}
}

func TestResponse_OmitsErrorWhenOK(t *testing.T) {
	t.Parallel()
	resp := Response{OK: true}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// error field should be absent
	var raw map[string]interface{}
	_ = json.Unmarshal(b, &raw)
	if _, has := raw["error"]; has {
		t.Errorf("error field present when OK=true and no error set")
	}
}
