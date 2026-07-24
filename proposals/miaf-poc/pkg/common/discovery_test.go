package common_test

import (
	"encoding/json"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
)

// Mirrors the SUP §5 "Example: Discovery Document" response.
const sampleDiscovery = `{
  "trustDomain": "northstar-ida.com",
  "trustBundleUri": "https://mis.northstar-ida.com/.well-known/spiffe/bundle.json",
  "margoIdentityServiceBaseUri": "https://mis.northstar-ida.com",
  "supportedBootstrapMethods": [
    "urn:margo:bootstrap:factory-cert-mtls:v1",
    "urn:margo:bootstrap:factory-cert-jwt:v1",
    "urn:margo:bootstrap:fdo:v1",
    "urn:margo:bootstrap:enrollment-token:v1"
  ],
  "svidProfilesSupported": [
    "https://margo.org/profiles/spiffe/x509-svid/v1",
    "https://margo.org/profiles/spiffe/jwt-svid/v1"
  ]
}`

func TestDiscoveryResponseDTO_RoundTrip(t *testing.T) {
	var dto common.DiscoveryResponseDTO
	if err := json.Unmarshal([]byte(sampleDiscovery), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got, want := dto.TrustDomain, "northstar-ida.com"; got != want {
		t.Errorf("trustDomain = %q, want %q", got, want)
	}
	if got, want := dto.MargoIdentityServiceBaseURI, "https://mis.northstar-ida.com"; got != want {
		t.Errorf("margoIdentityServiceBaseUri = %q, want %q", got, want)
	}
	if len(dto.SupportedBootstrapMethods) != 4 {
		t.Errorf("supportedBootstrapMethods len = %d, want 4", len(dto.SupportedBootstrapMethods))
	}
	if len(dto.SVIDProfilesSupported) != 2 {
		t.Fatalf("svidProfilesSupported len = %d, want 2", len(dto.SVIDProfilesSupported))
	}
	if got, want := dto.SVIDProfilesSupported[0], common.SVIDProfileURIX509V1; got != want {
		t.Errorf("profile[0] = %q, want %q", got, want)
	}

	// Marshal back and ensure the JSON is structurally equivalent (order-insensitive).
	out, err := json.Marshal(&dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var normIn, normOut any
	_ = json.Unmarshal([]byte(sampleDiscovery), &normIn)
	_ = json.Unmarshal(out, &normOut)
	if !jsonDeepEqual(normIn, normOut) {
		t.Errorf("round trip lost information:\nin:  %s\nout: %s", sampleDiscovery, out)
	}
}

func jsonDeepEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
