// Package agentstate holds device-agent local identity state.
package agentstate

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// State persists the device's current MIAF identity and daemon-side cache
// validators between invocations. New fields are optional - pre-daemon state
// files load with zero values for the daemon-only fields.
type State struct {
	// Core identity (set by enrollment / renewal).
	SPIFFEID     string    `json:"spiffe_id"`
	Serial       string    `json:"serial"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastEnrolled time.Time `json:"last_enrolled_at"`

	// Daemon-only cache validators (set by the IdentityManager).
	BundleLastModified      string    `json:"bundle_last_modified,omitempty"`
	RevocationsETag         string    `json:"revocations_etag,omitempty"`
	RevocationsLastModified string    `json:"revocations_last_modified,omitempty"`
	LastBundleRefreshAt     time.Time `json:"last_bundle_refresh_at,omitempty"`
	LastRevocationsCheckAt  time.Time `json:"last_revocations_check_at,omitempty"`
	LastRenewalAt           time.Time `json:"last_renewal_at,omitempty"`

	// Self-revocation flag (set by the IdentityManager when own serial is
	// observed in the revocation list). Zero value means not revoked.
	RevokedAt time.Time `json:"revoked_at,omitempty"`
}

// Save writes s to path with 0600 permissions, creating parent directories as
// needed. The write is atomic: bytes are first written to path+".tmp" and then
// renamed to path. A crash between the temp-write and the rename leaves the
// previous state file intact.
func Save(path string, s State) error {
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Load reads path into a State. Missing files return a descriptive error.
func Load(path string) (State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	return s, nil
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
