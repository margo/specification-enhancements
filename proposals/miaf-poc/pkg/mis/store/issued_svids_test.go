package store_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIssuedSVIDs_RecordAndActive(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	// Seed an LDI so the FK holds.
	ldi, err := s.Identities().CreateLDIAndBind(ctx, []byte("esi1aaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "urn:margo:bootstrap:factory-cert-mtls:v1", "margo.example.com")
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}

	log := s.IssuedSVIDs()
	if err := log.Record(ctx, identity.IssuedSVIDRecord{
		SerialHex:      "ABCDEF",
		FingerprintHex: "aa",
		SPIFFEID:       ldi.SPIFFEID,
		LDIUUID:        ldi.UUID,
		Method:         "urn:margo:bootstrap:factory-cert-mtls:v1",
		IssuedAt:       now,
		NotBefore:      now,
		NotAfter:       now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Idempotent: recording the same serial again is a no-op.
	if err := log.Record(ctx, identity.IssuedSVIDRecord{
		SerialHex: "ABCDEF", FingerprintHex: "aa", SPIFFEID: ldi.SPIFFEID, LDIUUID: ldi.UUID,
		Method: "urn:margo:bootstrap:factory-cert-mtls:v1", IssuedAt: now, NotBefore: now, NotAfter: now.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("Record (idempotent): %v", err)
	}

	active, err := log.ActiveBySPIFFEID(ctx, ldi.SPIFFEID, now)
	if err != nil {
		t.Fatalf("ActiveBySPIFFEID: %v", err)
	}
	if len(active) != 1 || active[0].SerialHex != "ABCDEF" {
		t.Fatalf("active = %+v", active)
	}

	if err := log.MarkRevoked(ctx, "ABCDEF", now.Add(time.Hour)); err != nil {
		t.Fatalf("MarkRevoked: %v", err)
	}

	active2, _ := log.ActiveBySPIFFEID(ctx, ldi.SPIFFEID, now)
	if len(active2) != 0 {
		t.Errorf("expected 0 active after revoke, got %+v", active2)
	}
}

func TestIssuedSVIDs_LeafByActiveSPIFFEID(t *testing.T) {
	ctx := context.Background()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	repo := s.Identities()
	created, err := repo.CreateLDIAndBind(ctx, make([]byte, 32),
		"urn:margo:bootstrap:factory-cert-mtls:v1", "margo.example.com")
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}

	leafDER := []byte{0x30, 0x01, 0x00} // any non-empty bytes
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	if err := s.IssuedSVIDs().Record(ctx, identity.IssuedSVIDRecord{
		SerialHex:      "ABCDEF01",
		FingerprintHex: "deadbeef",
		SPIFFEID:       created.SPIFFEID,
		LDIUUID:        created.UUID,
		Method:         "renewal",
		IssuedAt:       now,
		NotBefore:      now,
		NotAfter:       now.Add(time.Hour),
		LeafDER:        leafDER,
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := s.IssuedSVIDs().LeafByActiveSPIFFEID(ctx, created.SPIFFEID, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("LeafByActiveSPIFFEID: %v", err)
	}
	if !bytes.Equal(got.LeafDER, leafDER) {
		t.Errorf("LeafDER round-trip mismatch")
	}
}

func TestIssuedSVIDs_LeafByActiveSPIFFEID_ExpiredOrRevoked(t *testing.T) {
	ctx := context.Background()
	s, _ := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	t.Cleanup(func() { _ = s.Close() })

	repo := s.Identities()
	created, _ := repo.CreateLDIAndBind(ctx, make([]byte, 32),
		"urn:margo:bootstrap:factory-cert-mtls:v1", "margo.example.com")

	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	_ = s.IssuedSVIDs().Record(ctx, identity.IssuedSVIDRecord{
		SerialHex:      "01", FingerprintHex: "01", SPIFFEID: created.SPIFFEID,
		LDIUUID: created.UUID, Method: "renewal",
		IssuedAt: now.Add(-2 * time.Hour), NotBefore: now.Add(-2 * time.Hour),
		NotAfter: now.Add(-time.Hour), // expired
		LeafDER:  []byte{0x01},
	})
	if _, err := s.IssuedSVIDs().LeafByActiveSPIFFEID(ctx, created.SPIFFEID, now); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("expected ErrNotFound for expired-only history, got %v", err)
	}
}
