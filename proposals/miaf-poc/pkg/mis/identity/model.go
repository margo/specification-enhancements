// Package identity owns MIAF-specific LDI / PDI / ESI state.
package identity

import "time"

// LDIStatus tracks the high-level lifecycle of a Logical Device Identity.
// Revoked and Retired are terminal; per SUP §4 they MUST NOT be re-issued.
type LDIStatus string

const (
	LDIStatusActive  LDIStatus = "active"
	LDIStatusRevoked LDIStatus = "revoked"
	LDIStatusRetired LDIStatus = "retired"
)

// LDI is a Logical Device Identity record. The SPIFFE ID is derived from
// UUID: "spiffe://<trust-domain>/margo/device/<UUID>".
type LDI struct {
	UUID      string // RFC 4122 version 4, lowercase hex with hyphens
	SPIFFEID  string
	Status    LDIStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ESIBindingStatus distinguishes the active binding for an LDI from any
// previous bindings retired by a replacement event.
type ESIBindingStatus string

const (
	ESIBindingActive  ESIBindingStatus = "active"
	ESIBindingRetired ESIBindingStatus = "retired"
)

// ESIBinding links a method-derived ESI to an LDI.
type ESIBinding struct {
	ESI       []byte // raw 32-byte SHA-256 digest
	LDIUUID   string
	Method    string // bootstrap method URN
	Status    ESIBindingStatus
	BoundAt   time.Time
	RetiredAt time.Time // zero if Status == Active
}
