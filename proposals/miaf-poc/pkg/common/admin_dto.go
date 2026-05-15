package common

// CreateEnrollmentTokenRequest is the body of POST /admin/v1/enrollment-tokens.
//
// NON-NORMATIVE: this is a PoC administration API and is NOT part of the
// Margo Identity & Authorisation Framework SUP.
type CreateEnrollmentTokenRequest struct {
	// TTL in Go duration format, e.g. "1h", "15m". Required; positive.
	TTL string `json:"ttl"`
}

// CreateEnrollmentTokenResponse is returned on 201 Created.
type CreateEnrollmentTokenResponse struct {
	TokenID   string `json:"tokenId"`
	Secret    string `json:"secret"`
	ExpiresAt string `json:"expiresAt"` // RFC3339Nano
}

// CreateReplacementTicketRequest is the body of POST /admin/v1/replacement-tickets.
//
// NON-NORMATIVE: this is a PoC administration API and is NOT part of the
// Margo Identity & Authorisation Framework SUP.
type CreateReplacementTicketRequest struct {
	SPIFFEID string `json:"spiffeId"`
	TTL      string `json:"ttl"`
}

// CreateReplacementTicketResponse is returned on 201 Created.
type CreateReplacementTicketResponse struct {
	Ticket    string `json:"ticket"`
	SPIFFEID  string `json:"spiffeId"`
	ExpiresAt string `json:"expiresAt"`
}

// RevokeSPIFFEIDRequest is the body of POST /admin/v1/revocations.
//
// NON-NORMATIVE: this is a PoC administration API and is NOT part of the
// Margo Identity & Authorisation Framework SUP.
type RevokeSPIFFEIDRequest struct {
	SPIFFEID string `json:"spiffeId"`
	// Reason is an operator-supplied free-text label recorded on each
	// revoked entry. Optional.
	Reason string `json:"reason,omitempty"`
}

// RevokeSPIFFEIDResponse is the response body of POST /admin/v1/revocations.
type RevokeSPIFFEIDResponse struct {
	SPIFFEID       string   `json:"spiffeId"`
	RevokedSerials []string `json:"revokedSerials"`
	AlreadyRevoked []string `json:"alreadyRevoked,omitempty"`
}
