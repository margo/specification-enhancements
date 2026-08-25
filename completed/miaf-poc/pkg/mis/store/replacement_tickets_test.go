package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

func openRepl(t *testing.T) (*store.Store, identity.ReplacementTicketStore) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, s.ReplacementTickets()
}

// seedLDI inserts an LDI row so foreign-key constraints are satisfied.
func seedLDI(t *testing.T, s *store.Store) string {
	t.Helper()
	ldi, err := s.Identities().CreateLDIAndBind(
		context.Background(), []byte("00000000000000000000000000000000"),
		"urn:margo:bootstrap:factory-cert-mtls:v1", "margo.example.com")
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}
	return ldi.UUID
}

func TestReplacementTickets_CreateAndValidate(t *testing.T) {
	s, r := openRepl(t)
	ldiUUID := seedLDI(t, s)

	c, err := r.Create(context.Background(), ldiUUID, time.Hour, time.Now())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.Ticket == "" || c.ExpiresAt.IsZero() {
		t.Errorf("Create returned empty: %+v", c)
	}

	got, err := r.Validate(context.Background(), c.Ticket, time.Now())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got != ldiUUID {
		t.Errorf("Validate returned %q, want %q", got, ldiUUID)
	}
}

func TestReplacementTickets_SingleUse(t *testing.T) {
	s, r := openRepl(t)
	ldiUUID := seedLDI(t, s)
	c, _ := r.Create(context.Background(), ldiUUID, time.Hour, time.Now())

	if _, err := r.Validate(context.Background(), c.Ticket, time.Now()); err != nil {
		t.Fatalf("first Validate: %v", err)
	}
	if _, err := r.Validate(context.Background(), c.Ticket, time.Now()); !errors.Is(err, identity.ErrReplacementNotAuthorized) {
		t.Errorf("second Validate err = %v, want ErrReplacementNotAuthorized", err)
	}
}

func TestReplacementTickets_Expired(t *testing.T) {
	s, r := openRepl(t)
	ldiUUID := seedLDI(t, s)
	c, _ := r.Create(context.Background(), ldiUUID, time.Millisecond, time.Now().Add(-time.Hour))
	_, err := r.Validate(context.Background(), c.Ticket, time.Now())
	if !errors.Is(err, identity.ErrReplacementNotAuthorized) {
		t.Errorf("expired Validate err = %v, want ErrReplacementNotAuthorized", err)
	}
}

func TestReplacementTickets_Unknown(t *testing.T) {
	_, r := openRepl(t)
	_, err := r.Validate(context.Background(), "margo-rt-v1.AAAAAAAAAAAAAAAAAAAAAA", time.Now())
	if !errors.Is(err, identity.ErrReplacementNotAuthorized) {
		t.Errorf("unknown Validate err = %v, want ErrReplacementNotAuthorized", err)
	}
}
