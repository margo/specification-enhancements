package common

// RevocationListDTO is the response body of GET /api/v1/revocations per
// SUP §"Revocation List Endpoint". Field semantics:
//
//   - LastUpdated: max revoked_at across entries, or the empty string when
//     the list is empty. ISO 8601 UTC, second precision (e.g.
//     "2025-10-25T14:12:31Z"), matching the SUP example.
//   - RevokedSVIDs: always an empty array when no entries - never null.
type RevocationListDTO struct {
	LastUpdated  string             `json:"lastUpdated"`
	RevokedSVIDs []RevokedSVIDEntry `json:"revokedSvids"`
}

// RevokedSVIDEntry is one entry in RevocationListDTO.RevokedSVIDs.
//
//   - CertFingerprintSHA256: lowercase hex SHA-256 of the DER-encoded leaf.
//     SUP: consumers MUST use this as the primary identifier.
//   - SerialNumber: uppercase hex of the X.509 serial without prefixes or
//     delimiters. SUP warns serials are not globally unique across issuers.
//   - RevokedAt: ISO 8601 UTC, second precision.
type RevokedSVIDEntry struct {
	CertFingerprintSHA256 string `json:"certFingerprintSha256"`
	SerialNumber          string `json:"serialNumber"`
	RevokedAt             string `json:"revokedAt"`
}
