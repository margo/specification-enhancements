package errors_test

import (
	"encoding/json"
	"testing"

	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
)

func TestProblemDetails_AboutBlank(t *testing.T) {
	pd := errpkg.New(401, "Unauthorized").WithDetail("credential invalid")
	b, err := json.Marshal(pd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"type":"about:blank","title":"Unauthorized","status":401,"detail":"credential invalid"}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

func TestProblemDetails_MargoSpecific(t *testing.T) {
	pd := errpkg.UnsupportedBootstrapMethod.Problem("urn:unknown:bootstrap:x:v1 is not supported")
	b, _ := json.Marshal(pd)
	const want = `{"type":"https://margo.org/docs/errors/unsupported-method","title":"Unsupported Bootstrap Method","status":422,"detail":"urn:unknown:bootstrap:x:v1 is not supported"}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

func TestProblemDetails_Instance(t *testing.T) {
	pd := errpkg.New(500, "Internal").WithInstance("urn:uuid:00000000-0000-0000-0000-000000000001")
	b, _ := json.Marshal(pd)
	const want = `{"type":"about:blank","title":"Internal","status":500,"instance":"urn:uuid:00000000-0000-0000-0000-000000000001"}`
	if string(b) != want {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

func TestRegistry_AllSUPTypes(t *testing.T) {
	// SUP Appendix B "Margo-Specific Protocol Errors" table - every row.
	cases := []struct {
		spec    errpkg.Spec
		status  int
		typeURI string
	}{
		{errpkg.InvalidSVIDRequest, 400, "https://margo.org/docs/errors/invalid-svid-request"},
		{errpkg.InvalidBootstrapProof, 401, "https://margo.org/docs/errors/invalid-bootstrap-proof"},
		{errpkg.InvalidAudience, 400, "https://margo.org/docs/errors/invalid-audience"},
		{errpkg.UnsupportedBootstrapMethod, 422, "https://margo.org/docs/errors/unsupported-method"},
		{errpkg.UnsupportedReplacementAuthorizationMethod, 422, "https://margo.org/docs/errors/unsupported-replacement-authorization-method"},
		{errpkg.UnsupportedSVIDProfile, 422, "https://margo.org/docs/errors/unsupported-svid-profile"},
		{errpkg.SpuriousReplacementAuthorization, 400, "https://margo.org/docs/errors/spurious-replacement-authorization"},
		{errpkg.ReplacementNotAuthorized, 403, "https://margo.org/docs/errors/replacement-not-authorized"},
		{errpkg.KeyRotationNotPermitted, 409, "https://margo.org/docs/errors/key-rotation-not-permitted"},
		{errpkg.TooManyRequests, 429, "https://margo.org/docs/errors/too-many-requests"},
		{errpkg.InvalidEnrollmentToken, 401, "https://margo.org/docs/errors/invalid-enrollment-token"},
		{errpkg.RevocationFormat, 500, "https://margo.org/docs/errors/revocation-format"},
	}
	for _, c := range cases {
		if c.spec.Status != c.status {
			t.Errorf("%s: status = %d, want %d", c.spec.TypeURI, c.spec.Status, c.status)
		}
		if c.spec.TypeURI != c.typeURI {
			t.Errorf("status %d: typeURI = %q, want %q", c.status, c.spec.TypeURI, c.typeURI)
		}
	}
}
