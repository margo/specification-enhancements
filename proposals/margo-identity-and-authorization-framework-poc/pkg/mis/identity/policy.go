package identity

// PolicyEngine decides whether the MIS may create new LDIs or rotate keys.
// Real deployments will back this with a rules engine or admin policy store;
// the PoC's toggles are enough to cover the SUP error rows.
type PolicyEngine interface {
	// AllowNewLDI reports whether a fresh LDI may be minted for an unknown
	// ESI. If false, the enrollment handler returns 403 about:blank.
	AllowNewLDI() bool

	// AllowKeyRotation reports whether an existing LDI may be re-issued
	// with a new public key. If false, the enrollment handler returns 409
	// key-rotation-not-permitted per SUP Appendix B.
	AllowKeyRotation() bool
}

// StaticPolicy is a PolicyEngine with fixed answers. Used by the CLI and unit
// tests.
type StaticPolicy struct {
	NewLDI      bool
	KeyRotation bool
}

// AllowNewLDI implements PolicyEngine.
func (p StaticPolicy) AllowNewLDI() bool { return p.NewLDI }

// AllowKeyRotation implements PolicyEngine.
func (p StaticPolicy) AllowKeyRotation() bool { return p.KeyRotation }

// DefaultPolicy allows both new LDIs and key rotation. It is the PoC default.
func DefaultPolicy() PolicyEngine {
	return StaticPolicy{NewLDI: true, KeyRotation: true}
}
