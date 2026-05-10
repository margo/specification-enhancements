package agentstate_test

import (
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	want := agentstate.State{
		SPIFFEID:     "spiffe://margo.example.com/margo/device/00000000-0000-4000-8000-000000000001",
		Serial:       big.NewInt(42).Text(10),
		ExpiresAt:    time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC),
		LastEnrolled: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := agentstate.Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(raw), "privateKeyPem") {
		t.Fatalf("state.json should not persist privateKeyPem:\n%s", raw)
	}
	got, err := agentstate.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SPIFFEID != want.SPIFFEID || got.Serial != want.Serial {
		t.Errorf("round trip mismatch: %+v vs %+v", got, want)
	}
}

func TestLoad_Missing(t *testing.T) {
	_, err := agentstate.Load(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSaveAtomic_NoTmpFileLeftBehind(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := agentstate.Save(path, agentstate.State{SPIFFEID: "spiffe://td/margo/device/abc"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("found stray temp file: %s", e.Name())
		}
	}
}

func TestSaveAtomic_DoesNotPartiallyOverwriteOnEncodeFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	original := agentstate.State{SPIFFEID: "spiffe://td/margo/device/A", Serial: "111"}
	if err := agentstate.Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := agentstate.Save(path, agentstate.State{SPIFFEID: "spiffe://td/margo/device/B", Serial: "222"}); err != nil {
		t.Fatalf("Save (overwrite): %v", err)
	}
	got, err := agentstate.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Serial != "222" {
		t.Errorf("Serial = %q, want 222", got.Serial)
	}
}

func TestState_RoundTripsAllFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	now := time.Now().UTC().Truncate(time.Second)
	want := agentstate.State{
		SPIFFEID:                "spiffe://margo.example.com/margo/device/abc",
		Serial:                  "deadbeef",
		ExpiresAt:               now.Add(24 * time.Hour),
		LastEnrolled:            now,
		BundleLastModified:      "Wed, 23 Apr 2026 12:00:00 GMT",
		RevocationsETag:         `W/"abc123"`,
		RevocationsLastModified: "Wed, 23 Apr 2026 12:05:00 GMT",
		LastBundleRefreshAt:     now,
		LastRevocationsCheckAt:  now,
		LastRenewalAt:           now,
		// RevokedAt left zero on purpose
	}
	if err := agentstate.Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := agentstate.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch.\n got: %+v\nwant: %+v", got, want)
	}
}

func TestState_LoadsLegacyFileWithMissingDaemonFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	legacy := []byte(`{
      "spiffeId": "spiffe://td/margo/device/x",
      "serial": "1",
      "expiresAt": "2026-04-30T00:00:00Z",
      "privateKeyPem": "PEM",
      "lastEnrolledAt": "2026-04-23T00:00:00Z"
    }`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := agentstate.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.SPIFFEID != "spiffe://td/margo/device/x" {
		t.Errorf("SPIFFEID = %q", got.SPIFFEID)
	}
	if !got.RevokedAt.IsZero() {
		t.Errorf("RevokedAt should be zero on legacy file, got %v", got.RevokedAt)
	}
}
