package common

// JWTSVIDExchangeRequestDTO is the request body of
// POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid as defined in SUP
// §5 "JWT SVID Exchange Endpoint".
type JWTSVIDExchangeRequestDTO struct {
	// Audience is the requested aud claim of the issued JWT SVID. Required;
	// non-empty.
	Audience []string `json:"aud"`
	// TTLSeconds is the requested lifetime in seconds. Optional; capped by
	// MIS policy.
	TTLSeconds int `json:"ttl,omitempty"`
	// ClientAssertion is an optional JWT for client authentication when mTLS
	// is not used. Per SUP §5 "JWT SVID Exchange Endpoint" the assertion's
	// iss/sub MUST equal the SPIFFE ID, aud MUST equal the exchange endpoint
	// URL, exp MUST NOT exceed iat + 5 minutes, and jti MUST be unique.
	ClientAssertion string `json:"clientAssertion,omitempty"`
}

// JWTSVIDExchangeResponseDTO is the response body on 201 Created.
type JWTSVIDExchangeResponseDTO struct {
	// JWT is the compact JWT SVID. Its `sub` claim equals the SPIFFE ID
	// identified by {spiffeIdEncoded}.
	JWT string `json:"jwt"`
	// ExpiresAt is the UTC timestamp when the JWT SVID expires (ISO 8601).
	// Optional per SUP; clients may instead derive expiry from the JWT's
	// exp claim. The PoC always populates it for client convenience.
	ExpiresAt string `json:"expiresAt,omitempty"`
}
