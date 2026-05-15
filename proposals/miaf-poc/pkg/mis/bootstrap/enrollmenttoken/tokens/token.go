package tokens

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
)

// WirePrefix is the cosmetic marker on enrollment tokens, matching the SUP
// example in Appendix A ("margo-et-v1.<base64>"). It carries no security
// meaning - the 16 random bytes after it are what gate access - but it
// aids human identification of the credential class.
const WirePrefix = "margo-et-v1."

// SecretLenBytes is the length of the raw random secret body. 16 bytes =
// 128 bits, which satisfies SUP Appendix A "Tokens MUST have at least 128
// bits of cryptographic randomness".
const SecretLenBytes = 16

// Status values for a token row.
const (
	StatusActive   = "active"
	StatusConsumed = "consumed"
)

// Errors returned by parsing and by the Store contract.
var (
	ErrMalformedWire      = errors.New("tokens: malformed wire format")
	ErrNotFound           = errors.New("tokens: not found")
	ErrExpired            = errors.New("tokens: expired")
	ErrAlreadyUsed        = errors.New("tokens: already consumed")
	ErrOutcomeExists      = errors.New("tokens: outcome already recorded")
	ErrRetryWindowExpired = errors.New("tokens: retry window expired")
)

// Secret is a parsed enrollment-token secret. The zero value is invalid -
// construct via NewSecret or ParseSecret.
type Secret struct {
	raw [SecretLenBytes]byte
}

// NewSecret draws SecretLenBytes of entropy from r and returns a Secret.
func NewSecret(r io.Reader) (Secret, error) {
	var s Secret
	if _, err := io.ReadFull(r, s.raw[:]); err != nil {
		return Secret{}, fmt.Errorf("tokens: read entropy: %w", err)
	}
	return s, nil
}

// ParseSecret decodes a wire-format secret.
func ParseSecret(wire string) (Secret, error) {
	if !strings.HasPrefix(wire, WirePrefix) {
		return Secret{}, fmt.Errorf("%w: missing %q prefix", ErrMalformedWire, WirePrefix)
	}
	body := strings.TrimPrefix(wire, WirePrefix)
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return Secret{}, fmt.Errorf("%w: base64: %w", ErrMalformedWire, err)
	}
	if len(raw) != SecretLenBytes {
		return Secret{}, fmt.Errorf("%w: got %d bytes, want %d", ErrMalformedWire, len(raw), SecretLenBytes)
	}
	var s Secret
	copy(s.raw[:], raw)
	return s, nil
}

// Bytes returns a copy of the raw secret material. Callers MUST treat it
// as sensitive and zero it when done.
func (s Secret) Bytes() []byte {
	out := make([]byte, SecretLenBytes)
	copy(out, s.raw[:])
	return out
}

// Wire returns the on-the-wire representation.
func (s Secret) Wire() string {
	return WirePrefix + base64.RawURLEncoding.EncodeToString(s.raw[:])
}

// Hash returns SHA-256 of the raw secret. This is what the store indexes;
// the raw secret is never persisted.
func (s Secret) Hash() []byte {
	sum := sha256.Sum256(s.raw[:])
	return sum[:]
}

// NewTokenID returns a fresh random UUIDv4 suitable for use as the
// MIS-assigned token_id. Independence from the secret (SUP Appendix A
// Token Requirement #5) is ensured by using a fresh OS-supplied random
// draw inside uuid.New - not a derivation of the secret.
func NewTokenID() string {
	return uuid.New().String()
}

// Outcome is what RecordOutcome persists so Consume+Outcome form the
// material for CheckReplay. Retained only for the duration of the
// retry window.
type Outcome struct {
	TokenID             string
	StatusCode          int
	CertificateChainPEM []string // as served in the response
	SVIDProfileURI      string
	CSRPubKeySHA256     []byte
	LDIUUID             string
	CompletedAt         time.Time
	RetryWindowEnds     time.Time
}
