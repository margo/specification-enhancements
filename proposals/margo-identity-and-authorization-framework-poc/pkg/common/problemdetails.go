// Package common - shared types between MIS and device-agent.
// pkg/mis/errors holds the server-side registry; this is the device-side
// decoder. Kept in pkg/common because both MIS and device-agent share these
// wire types.
package common

// ProblemDetails mirrors the JSON object MIS emits per RFC 9457 and SUP
// Appendix B. All fields are optional from the wire perspective; the daemon's
// manager consults Type and Status to decide retry behavior.
type ProblemDetails struct {
	Type     string `json:"type,omitempty"` // URI; "about:blank" when generic
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"` // urn:uuid:...
}

// IsMargoSpecific returns true when Type points at the margo.org error
// namespace. Used by the daemon to decide whether to consult the per-error
// retry table or fall back to status-only handling.
func (p ProblemDetails) IsMargoSpecific() bool {
	const ns = "https://margo.org/docs/errors/"
	return len(p.Type) >= len(ns) && p.Type[:len(ns)] == ns
}
