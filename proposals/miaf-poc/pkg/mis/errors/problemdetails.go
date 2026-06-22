// Package errors implements RFC 9457 Problem Details responses for the MIS
// per SUP Appendix B.
//
// The file defines two things:
//
//   - a generic ProblemDetails type with JSON marshaling that matches RFC 9457,
//   - a registry of the Margo-specific error types enumerated in SUP Appendix B.
//
// The SUP is the single source of truth for the Margo-specific error-type
// registry; keep this file in sync with Appendix B.
package errors

// ContentType is the RFC 9457 media type for Problem Details responses.
const ContentType = "application/problem+json"

// ProblemDetails is an RFC 9457 response body. All fields except "type",
// "title", and "status" are optional and omitted when empty.
type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// New constructs a Problem Details object for a generic HTTP error. The
// "type" field is set to "about:blank" per RFC 9457 §4.2.
func New(status int, title string) *ProblemDetails {
	return &ProblemDetails{Type: "about:blank", Title: title, Status: status}
}

// WithDetail attaches developer-readable detail.
func (p *ProblemDetails) WithDetail(detail string) *ProblemDetails {
	p.Detail = detail
	return p
}

// WithInstance attaches a per-occurrence URI (correlation / audit ID).
func (p *ProblemDetails) WithInstance(instance string) *ProblemDetails {
	p.Instance = instance
	return p
}

// Spec is a Margo-specific error type from SUP Appendix B.
type Spec struct {
	Status  int
	TypeURI string
	Title   string
}

// Problem materializes a ProblemDetails response from a Spec, with optional
// developer-readable detail.
func (s Spec) Problem(detail string) *ProblemDetails {
	pd := &ProblemDetails{Type: s.TypeURI, Title: s.Title, Status: s.Status}
	if detail != "" {
		pd.Detail = detail
	}
	return pd
}

// Registry of Margo-specific error types defined in SUP Appendix B.
//
// This registry mirrors the Margo-specific error types defined in SUP
// Appendix B.
var (
	InvalidSVIDRequest = Spec{
		Status:  400,
		TypeURI: "https://margo.org/docs/errors/invalid-svid-request",
		Title:   "Invalid SVID Request",
	}
	InvalidBootstrapProof = Spec{
		Status:  401,
		TypeURI: "https://margo.org/docs/errors/invalid-bootstrap-proof",
		Title:   "Invalid Bootstrap Proof",
	}
	InvalidAudience = Spec{
		Status:  400,
		TypeURI: "https://margo.org/docs/errors/invalid-audience",
		Title:   "Invalid Audience",
	}
	UnsupportedBootstrapMethod = Spec{
		Status:  422,
		TypeURI: "https://margo.org/docs/errors/unsupported-method",
		Title:   "Unsupported Bootstrap Method",
	}
	UnsupportedReplacementAuthorizationMethod = Spec{
		Status:  422,
		TypeURI: "https://margo.org/docs/errors/unsupported-replacement-authorization-method",
		Title:   "Unsupported Replacement Authorization Method",
	}
	UnsupportedSVIDProfile = Spec{
		Status:  422,
		TypeURI: "https://margo.org/docs/errors/unsupported-svid-profile",
		Title:   "Unsupported SVID Profile",
	}
	SpuriousReplacementAuthorization = Spec{
		Status:  400,
		TypeURI: "https://margo.org/docs/errors/spurious-replacement-authorization",
		Title:   "Spurious Replacement Authorization",
	}
	ReplacementNotAuthorized = Spec{
		Status:  403,
		TypeURI: "https://margo.org/docs/errors/replacement-not-authorized",
		Title:   "Replacement Not Authorized",
	}
	KeyRotationNotPermitted = Spec{
		Status:  409,
		TypeURI: "https://margo.org/docs/errors/key-rotation-not-permitted",
		Title:   "Key Rotation Not Permitted",
	}
	TooManyRequests = Spec{
		Status:  429,
		TypeURI: "https://margo.org/docs/errors/too-many-requests",
		Title:   "Too Many Requests",
	}
	InvalidEnrollmentToken = Spec{
		Status:  401,
		TypeURI: "https://margo.org/docs/errors/invalid-enrollment-token",
		Title:   "Invalid Enrollment Token",
	}
	RevocationFormat = Spec{
		Status:  500,
		TypeURI: "https://margo.org/docs/errors/revocation-format",
		Title:   "Revocation List Parsing Error",
	}
)
