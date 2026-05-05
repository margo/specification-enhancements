// Package common holds types shared between MIS and the device-agent. These
// mirror the wire-format types defined in the MIAF SUP §5.
package common

// DiscoveryResponseDTO is the response body of GET /.well-known/margo
// as defined in SUP §5 "Discovery Document Endpoint".
type DiscoveryResponseDTO struct {
	TrustDomain                 string                  `json:"trust_domain"`
	TrustBundleURI              string                  `json:"trust_bundle_uri"`
	MargoIdentityServiceBaseURI string                  `json:"margo_identity_service_base_uri"`
	SupportedBootstrapMethods   []string                `json:"supported_bootstrap_methods"`
	SVIDProfilesSupported       []SVIDProfileDescriptor `json:"svid_profiles_supported"`
	RecommendedSVIDProfileURI   string                  `json:"recommended_svid_profile_uri"`
}

// SVIDProfileDescriptor describes one SVID profile advertised in the
// discovery document.
type SVIDProfileDescriptor struct {
	Type     string   `json:"type"`
	Versions []string `json:"versions"`
}

// Well-known SVID profile URIs for the Edge Compute Device Profile.
const (
	SVIDProfileURIX509V1 = "https://margo.org/profiles/spiffe/x509-svid/v1"
	SVIDProfileURIJWTV1  = "https://margo.org/profiles/spiffe/jwt-svid/v1"

	SVIDProfileTypeX509 = "x509-svid"
	SVIDProfileTypeJWT  = "jwt-svid"
)

// Well-known bootstrap method URNs from SUP Appendix A.
const (
	BootstrapMethodFactoryCertMTLS = "urn:margo:bootstrap:factory-cert-mtls:v1"
	BootstrapMethodFactoryCertJWT  = "urn:margo:bootstrap:factory-cert-jwt:v1"
	BootstrapMethodFDO             = "urn:margo:bootstrap:fdo:v1"
	BootstrapMethodEnrollmentToken = "urn:margo:bootstrap:enrollment-token:v1"
)
