package common_test

import (
	"encoding/json"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
)

// Mirrors the SUP §5 "Example: Enrollment and Identity Issuance" request body.
// The CSR base64 and bootstrap proof values are placeholders with the same
// structure as the SUP example (not intended to be a real CSR).
const sampleEnrollmentRequest = `{
  "svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1",
  "svid_request": {
    "csr": "MIICVzCCAT8CAQAwEjEQMA4GA1UEAwwHbWFyZ28="
  },
  "bootstrapCredential": {
    "method": "urn:margo:bootstrap:factory-cert-mtls:v1"
  }
}`

const sampleEnrollmentResponse = `{
  "svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1",
  "svid": {
    "certificate_chain_pem": [
      "-----BEGIN CERTIFICATE-----\nMIIC4TCCAcigAwIBAgIUFsO2...\n-----END CERTIFICATE-----",
      "-----BEGIN CERTIFICATE-----\nMIIDdTCCAl2gAwIBAgIURv7O...\n-----END CERTIFICATE-----"
    ]
  }
}`

func TestEnrollmentRequestDTO_RoundTrip(t *testing.T) {
	var req common.EnrollmentRequestDTO
	if err := json.Unmarshal([]byte(sampleEnrollmentRequest), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := req.SVIDProfileURI, common.SVIDProfileURIX509V1; got != want {
		t.Errorf("svid_profile_uri = %q, want %q", got, want)
	}
	if got, want := req.BootstrapCredential.Method, common.BootstrapMethodFactoryCertMTLS; got != want {
		t.Errorf("bootstrapCredential.method = %q, want %q", got, want)
	}
	if req.ReplacementAuthorization != nil {
		t.Errorf("replacementAuthorization must be nil by default, got %+v", req.ReplacementAuthorization)
	}
	if req.SVIDRequest.CSR == "" {
		t.Error("svid_request.csr missing")
	}

	// `proof` is omitempty; factory-cert-mtls omits it per SUP Appendix A.
	out, err := json.Marshal(&req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(out); got == "" || containsByteSeq(got, `"proof"`) {
		t.Errorf("proof should not appear when nil; got %s", got)
	}
}

func TestEnrollmentResponseDTO_RoundTrip(t *testing.T) {
	var resp common.EnrollmentResponseDTO
	if err := json.Unmarshal([]byte(sampleEnrollmentResponse), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, want := resp.SVIDProfileURI, common.SVIDProfileURIX509V1; got != want {
		t.Errorf("svid_profile_uri = %q, want %q", got, want)
	}
	if got := len(resp.SVID.CertificateChainPEM); got != 2 {
		t.Errorf("certificate_chain_pem len = %d, want 2", got)
	}
}

func TestReplacementAuthorizationDTO_OperatorTicket(t *testing.T) {
	const body = `{
	  "svid_profile_uri": "https://margo.org/profiles/spiffe/x509-svid/v1",
	  "svid_request": {"csr": "AAAA"},
	  "bootstrapCredential": {"method": "urn:margo:bootstrap:factory-cert-mtls:v1"},
	  "replacementAuthorization": {
	    "method": "urn:margo:replacement-auth:operator-ticket:v1",
	    "proof": {"ticket": "opaque-single-use-ticket"}
	  }
	}`
	var req common.EnrollmentRequestDTO
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.ReplacementAuthorization == nil {
		t.Fatal("replacementAuthorization must not be nil")
	}
	if got, want := req.ReplacementAuthorization.Method, common.ReplacementAuthMethodOperatorTicket; got != want {
		t.Errorf("method = %q, want %q", got, want)
	}
	if got := req.ReplacementAuthorization.Proof.Ticket; got != "opaque-single-use-ticket" {
		t.Errorf("ticket = %q", got)
	}
}

func containsByteSeq(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	}())
}
