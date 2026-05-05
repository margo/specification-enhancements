package conformance_test

import (
	"context"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// noopIssuanceLog is a no-op IssuanceLog for conformance tests that do not
// exercise the audit path.
type noopIssuanceLog struct{}

func (noopIssuanceLog) Record(_ context.Context, _ identity.IssuedSVIDRecord) error { return nil }
func (noopIssuanceLog) ActiveBySPIFFEID(_ context.Context, _ string, _ time.Time) ([]identity.IssuedSVIDRecord, error) {
	return nil, nil
}
func (noopIssuanceLog) MarkRevoked(_ context.Context, _ string, _ time.Time) error { return nil }
func (noopIssuanceLog) LeafByActiveSPIFFEID(_ context.Context, _ string, _ time.Time) (identity.IssuedSVIDRecord, error) {
	return identity.IssuedSVIDRecord{}, identity.ErrNotFound
}

// noopRevocationQueue is a no-op RevocationQueue for conformance tests that do
// not exercise the revocation path.
type noopRevocationQueue struct{}

func (noopRevocationQueue) Revoke(_ context.Context, _, _, _ string, _, _ time.Time) (bool, error) {
	return false, nil
}
func (noopRevocationQueue) List(_ context.Context, _ time.Time) ([]identity.RevocationEntry, time.Time, error) {
	return nil, time.Time{}, nil
}
func (noopRevocationQueue) PruneExpired(_ context.Context, _ time.Time) (int, error) { return 0, nil }
