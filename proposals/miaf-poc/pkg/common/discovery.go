// Package common holds types shared between MIS and the device-agent. These
// mirror the wire-format types defined in the MIAF SUP §5.
package common

// DiscoveryResponseDTO is the response body of GET /.well-known/margo
// as defined in SUP §5 "Discovery Document Endpoint".
type DiscoveryResponseDTO struct {
	TrustDomain                 string   `json:"trustDomain"`
	TrustBundleURI              string   `json:"trustBundleUri"`
	MargoIdentityServiceBaseURI string   `json:"margoIdentityServiceBaseUri"`
	SupportedBootstrapMethods   []string `json:"supportedBootstrapMethods"`
	SVIDProfilesSupported       []string `json:"svidProfilesSupported"`
}

// Well-known SVID profile URIs for the Edge Compute Device Profile.
const (
	SVIDProfileURIX509V1 = "https://margo.org/profiles/spiffe/x509-svid/v1"
	SVIDProfileURIJWTV1  = "https://margo.org/profiles/spiffe/jwt-svid/v1"
)

// Well-known bootstrap method URNs from SUP Appendix A.
const (
	BootstrapMethodFactoryCertMTLS = "urn:margo:bootstrap:factory-cert-mtls:v1"
	BootstrapMethodFactoryCertJWT  = "urn:margo:bootstrap:factory-cert-jwt:v1"
	BootstrapMethodFDO             = "urn:margo:bootstrap:fdo:v1"
	BootstrapMethodEnrollmentToken = "urn:margo:bootstrap:enrollment-token:v1"
)
