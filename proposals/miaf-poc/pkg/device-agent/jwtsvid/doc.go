// Package jwtsvid implements the device-agent side of the JWT SVID Exchange
// Endpoint (POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid).
//
// The package provides three entry points:
//   - BuildClientAssertion: builds a client_assertion JWT for non-mTLS auth.
//   - Exchange: posts the exchange request to MIS and returns the JWT SVID.
//   - verify.ParseAndValidate: validates the returned JWT SVID against the Trust Bundle.
package jwtsvid
