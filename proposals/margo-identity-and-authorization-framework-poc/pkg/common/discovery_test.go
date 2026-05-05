package common_test

import (
	"encoding/json"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
)

// Mirrors the SUP §5 "Example: Discovery Document" response.
const sampleDiscovery = `{
  "trust_domain": "northstar-ida.com",
  "trust_bundle_uri": "https://mis.northstar-ida.com/.well-known/spiffe/bundle.json",
  "margo_identity_service_base_uri": "https://mis.northstar-ida.com",
  "supported_bootstrap_methods": [
    "urn:margo:bootstrap:factory-cert-mtls:v1",
    "urn:margo:bootstrap:factory-cert-jwt:v1",
    "urn:margo:bootstrap:fdo:v1",
    "urn:margo:bootstrap:enrollment-token:v1"
  ],
  "svid_profiles_supported": [
    {"type": "x509-svid", "versions": ["https://margo.org/profiles/spiffe/x509-svid/v1"]},
    {"type": "jwt-svid",  "versions": ["https://margo.org/profiles/spiffe/jwt-svid/v1"]}
  ],
  "recommended_svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1"
}`

func TestDiscoveryResponseDTO_RoundTrip(t *testing.T) {
	var dto common.DiscoveryResponseDTO
	if err := json.Unmarshal([]byte(sampleDiscovery), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got, want := dto.TrustDomain, "northstar-ida.com"; got != want {
		t.Errorf("trust_domain = %q, want %q", got, want)
	}
	if got, want := dto.MargoIdentityServiceBaseURI, "https://mis.northstar-ida.com"; got != want {
		t.Errorf("margo_identity_service_base_uri = %q, want %q", got, want)
	}
	if len(dto.SupportedBootstrapMethods) != 4 {
		t.Errorf("supported_bootstrap_methods len = %d, want 4", len(dto.SupportedBootstrapMethods))
	}
	if len(dto.SVIDProfilesSupported) != 2 {
		t.Fatalf("svid_profiles_supported len = %d, want 2", len(dto.SVIDProfilesSupported))
	}
	if got, want := dto.SVIDProfilesSupported[0].Type, "x509-svid"; got != want {
		t.Errorf("profile[0].type = %q, want %q", got, want)
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
