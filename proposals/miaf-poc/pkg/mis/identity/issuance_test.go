package identity_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// memIssuanceLog is the in-memory IssuanceLog used across identity tests.
type memIssuanceLog struct {
	records []identity.IssuedSVIDRecord
}

func (m *memIssuanceLog) Record(_ context.Context, r identity.IssuedSVIDRecord) error {
	for i, ex := range m.records {
		if ex.SerialHex == r.SerialHex {
			m.records[i] = r
			return nil
		}
	}
	m.records = append(m.records, r)
	return nil
}

func (m *memIssuanceLog) ActiveBySPIFFEID(_ context.Context, spiffeID string, now time.Time) ([]identity.IssuedSVIDRecord, error) {
	var out []identity.IssuedSVIDRecord
	for _, r := range m.records {
		if r.SPIFFEID == spiffeID && !r.Revoked && r.NotAfter.After(now) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (m *memIssuanceLog) MarkRevoked(_ context.Context, serialHex string, when time.Time) error {
	for i, r := range m.records {
		if r.SerialHex == serialHex {
			m.records[i].Revoked = true
			m.records[i].RevokedAt = when
			return nil
		}
	}
	return identity.ErrNotFound
}

func (m *memIssuanceLog) LeafByActiveSPIFFEID(_ context.Context, spiffeID string, now time.Time) (identity.IssuedSVIDRecord, error) {
	var latest identity.IssuedSVIDRecord
	found := false
	for _, r := range m.records {
		if r.SPIFFEID == spiffeID && !r.Revoked && r.NotAfter.After(now) {
			if !found || r.IssuedAt.After(latest.IssuedAt) {
				latest = r
				found = true
			}
		}
	}
	if !found {
		return identity.IssuedSVIDRecord{}, identity.ErrNotFound
	}
	return latest, nil
}

func TestIssuanceLog_FakeInvariants(t *testing.T) {
	log := &memIssuanceLog{}
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	rec := identity.IssuedSVIDRecord{
		SerialHex:      "ABCDEF",
		FingerprintHex: "aa",
		SPIFFEID:       "spiffe://td/margo/device/1",
		LDIUUID:        "ldi-1",
		Method:         "urn:margo:bootstrap:factory-cert-mtls:v1",
		IssuedAt:       now,
		NotBefore:      now,
		NotAfter:       now.Add(24 * time.Hour),
	}
	if err := log.Record(ctx, rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	active, err := log.ActiveBySPIFFEID(ctx, rec.SPIFFEID, now)
	if err != nil {
		t.Fatalf("ActiveBySPIFFEID: %v", err)
	}
	if len(active) != 1 || active[0].SerialHex != "ABCDEF" {
		t.Errorf("active = %+v", active)
	}

	if err := log.MarkRevoked(ctx, "ABCDEF", now.Add(time.Hour)); err != nil {
		t.Fatalf("MarkRevoked: %v", err)
	}
	active2, _ := log.ActiveBySPIFFEID(ctx, rec.SPIFFEID, now)
	if len(active2) != 0 {
		t.Errorf("expected 0 active after revoke, got %+v", active2)
	}

	if err := log.MarkRevoked(ctx, "DOES-NOT-EXIST", now); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("MarkRevoked missing serial: got %v, want ErrNotFound", err)
	}

	expired := identity.IssuedSVIDRecord{
		SerialHex:      "EXPIRED",
		FingerprintHex: "bb",
		SPIFFEID:       rec.SPIFFEID,
		LDIUUID:        "ldi-1",
		Method:         "urn:margo:bootstrap:factory-cert-mtls:v1",
		IssuedAt:       now.Add(-48 * time.Hour),
		NotBefore:      now.Add(-48 * time.Hour),
		NotAfter:       now.Add(-time.Second),
	}
	_ = log.Record(ctx, expired)
	active3, _ := log.ActiveBySPIFFEID(ctx, rec.SPIFFEID, now)
	for _, a := range active3 {
		if a.SerialHex == "EXPIRED" {
			t.Error("expired entry returned as active")
		}
	}
}
