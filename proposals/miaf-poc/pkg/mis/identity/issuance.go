package identity

import (
	"context"
	"time"
)

// IssuedSVIDRecord is one row of the issuance audit. Every successful
// IssueX509SVID call in Service produces one of these; the store adapter
// persists them to the `issued_svids` table.
//
// SerialHex is the uppercase-hex form of the X.509 serial number, without
// prefixes or delimiters. This matches the `serial_number` wire field in
// SUP §"Revocation List Endpoint".
//
// FingerprintHex is the lowercase-hex SHA-256 digest of the DER-encoded
// leaf certificate. This matches `cert_fingerprint_sha256` in the SUP.
type IssuedSVIDRecord struct {
	SerialHex      string
	FingerprintHex string
	SPIFFEID       string
	LDIUUID        string
	Method         string // bootstrap method URN or "renewal"
	IssuedAt       time.Time
	NotBefore      time.Time
	NotAfter       time.Time
	Revoked        bool
	RevokedAt      time.Time
	// LeafDER is the DER-encoded leaf certificate. Required by Service.IssueJWTSVID
	// to recover the device's public key for client_assertion verification.
	LeafDER []byte
}

// IssuanceLog persists audit rows and answers "which SVIDs for this SPIFFE ID
// are still outstanding?".
type IssuanceLog interface {
	// Record inserts (or idempotently updates) a row for the given serial.
	Record(ctx context.Context, record IssuedSVIDRecord) error

	// ActiveBySPIFFEID returns all records for this SPIFFE ID that are
	// (1) not marked Revoked and (2) have not_after > now.
	ActiveBySPIFFEID(ctx context.Context, spiffeID string, now time.Time) ([]IssuedSVIDRecord, error)

	// MarkRevoked flips the Revoked/RevokedAt columns on the given serial.
	// Returns ErrNotFound when the serial is absent from the audit.
	MarkRevoked(ctx context.Context, serialHex string, when time.Time) error

	// LeafByActiveSPIFFEID returns the most recently issued, non-revoked,
	// not-yet-expired leaf record for spiffeID, or ErrNotFound when no such
	// record exists. Used by Renew to detect key rotation and by the JWT SVID
	// Exchange handler to recover the device's public key for client_assertion
	// verification.
	LeafByActiveSPIFFEID(ctx context.Context, spiffeID string, now time.Time) (IssuedSVIDRecord, error)
}
