package common

// EnrollmentRequestDTO is the request body of POST /api/v1/identities as
// defined in SUP §5 "Enrollment and Identity Issuance Endpoint".
type EnrollmentRequestDTO struct {
	SVIDProfileURI           string                       `json:"svid_profile_uri"`
	SVIDRequest              SVIDRequestX509DTO           `json:"svid_request"`
	BootstrapCredential      BootstrapCredentialDTO       `json:"bootstrapCredential"`
	ReplacementAuthorization *ReplacementAuthorizationDTO `json:"replacementAuthorization,omitempty"`
}

// SVIDRequestX509DTO is the svid_request payload for the X.509 SVID profile.
// Per SUP §4 "Edge Compute Device Identity Profile", X.509 is the only
// permitted profile for device enrollment; jwt-svid is rejected with 422.
type SVIDRequestX509DTO struct {
	// CSR is the device's PKCS#10 request, DER-encoded and standard
	// base64-encoded (no PEM armor, no newlines), per SUP §5.
	CSR string `json:"csr"`
}

// BootstrapCredentialDTO carries the method URN and (optional) method-specific
// proof. For factory-cert-mtls the proof MUST be omitted (the credential is
// the mTLS client certificate; see SUP Appendix A "Factory Certificate Method
// (mTLS)").
type BootstrapCredentialDTO struct {
	Method string          `json:"method"`
	Proof  *BootstrapProof `json:"proof,omitempty"`
}

// BootstrapProof is a method-specific proof bag. Fields not used by the
// selected method are omitted. factory-cert-jwt, enrollment-token, and FDO
// fill this in in later sub-projects.
type BootstrapProof struct {
	// Assertion is the Bootstrap Assertion JWT used by factory-cert-jwt.
	Assertion string `json:"assertion,omitempty"`
	// Token is the single-use enrollment token used by enrollment-token.
	Token string `json:"token,omitempty"`
}

// ReplacementAuthorizationDTO conveys policy-controlled authorization to
// rebind the request's derived ESI to an existing LDI. Absent for initial
// enrollment and ordinary re-enrollment.
type ReplacementAuthorizationDTO struct {
	Method string                        `json:"method"`
	Proof  ReplacementAuthorizationProof `json:"proof"`
}

// ReplacementAuthorizationProof holds the method-specific proof material.
// For urn:margo:replacement-auth:operator-ticket:v1, Ticket is required.
type ReplacementAuthorizationProof struct {
	Ticket string `json:"ticket,omitempty"`
}

// EnrollmentResponseDTO is the response body of POST /api/v1/identities for
// the X.509 SVID profile. 201 Created (initial) and 200 OK (re-enrollment,
// replacement) both use this shape; the status code distinguishes them.
type EnrollmentResponseDTO struct {
	SVIDProfileURI string      `json:"svid_profile_uri"`
	SVID           X509SVIDDTO `json:"svid"`
}

// X509SVIDDTO wraps the PEM-encoded leaf + intermediates returned to the
// device. Per SUP §5 the leaf is first and the root MAY be omitted.
type X509SVIDDTO struct {
	CertificateChainPEM []string `json:"certificate_chain_pem"`
}

// Replacement authorization method URNs defined by the SUP.
const (
	ReplacementAuthMethodOperatorTicket = "urn:margo:replacement-auth:operator-ticket:v1"
)
