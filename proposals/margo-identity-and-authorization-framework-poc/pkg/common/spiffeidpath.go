package common

import "encoding/base64"

// EncodeSPIFFEID encodes a SPIFFE ID as base64url with no padding, per SUP
// §"Common URI and Encoding Rules". Used to construct the {spiffeIdEncoded}
// path parameter for the SVID Renewal and JWT SVID Exchange endpoints.
func EncodeSPIFFEID(spiffeID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(spiffeID))
}
