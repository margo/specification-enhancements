package store_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"path/filepath"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

func freshStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identity.sqlite3")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func esiOf(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

func TestCreateLDIAndBind_Initial(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	repo := s.Identities()

	ldi, err := repo.CreateLDIAndBind(ctx, esiOf("a"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}
	if ldi.UUID == "" {
		t.Error("UUID must be assigned")
	}
	if ldi.SPIFFEID != "spiffe://margo.example.com/margo/device/"+ldi.UUID {
		t.Errorf("SPIFFEID = %q", ldi.SPIFFEID)
	}
	if ldi.Status != identity.LDIStatusActive {
		t.Errorf("Status = %q", ldi.Status)
	}

	got, _, err := repo.LookupByESI(ctx, esiOf("a"))
	if err != nil {
		t.Fatalf("LookupByESI: %v", err)
	}
	if got.Status != identity.ESIBindingActive {
		t.Errorf("binding status = %q", got.Status)
	}
}

func TestLookup_NotFound(t *testing.T) {
	s := freshStore(t)
	repo := s.Identities()
	_, _, err := repo.LookupByESI(context.Background(), esiOf("unknown"))
	if !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateLDIAndBind_ESIImmutability(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	repo := s.Identities()

	if _, err := repo.CreateLDIAndBind(ctx, esiOf("e"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := repo.CreateLDIAndBind(ctx, esiOf("e"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com"); err == nil {
		t.Error("expected error rebinding the same ESI to a second LDI (SUP §4)")
	}
}

func TestRebind_RetiresPrevious(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	repo := s.Identities()

	ldi, err := repo.CreateLDIAndBind(ctx, esiOf("old"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := repo.Rebind(ctx, ldi.UUID, esiOf("new"), common.BootstrapMethodFactoryCertMTLS); err != nil {
		t.Fatalf("Rebind: %v", err)
	}

	// Old ESI must be retired (still looked up, but marked retired).
	oldGot, _, err := repo.LookupByESI(ctx, esiOf("old"))
	if err != nil {
		t.Fatalf("LookupByESI old: %v", err)
	}
	if oldGot.Status != identity.ESIBindingRetired {
		t.Errorf("old binding status = %q, want retired", oldGot.Status)
	}

	// New ESI must be active and bound to the same LDI.
	got, gotLDI, err := repo.LookupByESI(ctx, esiOf("new"))
	if err != nil {
		t.Fatalf("LookupByESI new: %v", err)
	}
	if got.Status != identity.ESIBindingActive {
		t.Errorf("new binding status = %q", got.Status)
	}
	if gotLDI.UUID != ldi.UUID {
		t.Errorf("LDI mismatch: got %s, want %s", gotLDI.UUID, ldi.UUID)
	}

	bindings, err := repo.ListBindings(ctx, ldi.UUID)
	if err != nil {
		t.Fatalf("ListBindings: %v", err)
	}
	if len(bindings) != 2 {
		t.Errorf("bindings = %d, want 2", len(bindings))
	}
}

func TestRebind_LDIMustExist(t *testing.T) {
	s := freshStore(t)
	repo := s.Identities()
	_, err := repo.Rebind(context.Background(), "00000000-0000-4000-8000-000000000000", esiOf("x"), common.BootstrapMethodFactoryCertMTLS)
	if !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func mustOpen(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestIdentities_GetLDIBySPIFFEID(t *testing.T) {
	ctx := context.Background()
	s := mustOpen(t)
	repo := s.Identities()

	created, err := repo.CreateLDIAndBind(ctx, []byte("esi-bytes-32................................"[:32]), "urn:margo:bootstrap:factory-cert-mtls:v1", "margo.example.com")
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}

	got, err := repo.GetLDIBySPIFFEID(ctx, created.SPIFFEID)
	if err != nil {
		t.Fatalf("GetLDIBySPIFFEID: %v", err)
	}
	if got.UUID != created.UUID {
		t.Errorf("UUID = %q, want %q", got.UUID, created.UUID)
	}
}

func TestIdentities_GetLDIBySPIFFEID_NotFound(t *testing.T) {
	ctx := context.Background()
	s := mustOpen(t)
	repo := s.Identities()

	_, err := repo.GetLDIBySPIFFEID(ctx, "spiffe://margo.example.com/margo/device/does-not-exist")
	if !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("err = %v, want identity.ErrNotFound", err)
	}
}

func TestIdentities_SetLDIStatus(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	ldi, err := s.Identities().CreateLDIAndBind(ctx, []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"), "urn:margo:bootstrap:factory-cert-mtls:v1", "margo.example.com")
	if err != nil {
		t.Fatalf("CreateLDIAndBind: %v", err)
	}

	if err := s.Identities().SetLDIStatus(ctx, ldi.UUID, identity.LDIStatusRevoked); err != nil {
		t.Fatalf("SetLDIStatus: %v", err)
	}
	out, _ := s.Identities().GetLDI(ctx, ldi.UUID)
	if out.Status != identity.LDIStatusRevoked {
		t.Errorf("status = %q, want revoked", out.Status)
	}

	// Unknown LDI returns ErrNotFound.
	if err := s.Identities().SetLDIStatus(ctx, "no-such-ldi", identity.LDIStatusRevoked); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("SetLDIStatus unknown: got %v, want ErrNotFound", err)
	}
}
