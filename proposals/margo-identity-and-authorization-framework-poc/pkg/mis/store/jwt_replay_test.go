package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

func openReplay(t *testing.T) replay.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.JWTReplay()
}

func TestJWTReplay_FirstWitnessSucceeds(t *testing.T) {
	r := openReplay(t)
	if err := r.Witness(context.Background(), "jti-1", time.Now().Add(time.Minute)); err != nil {
		t.Errorf("Witness: %v", err)
	}
}

func TestJWTReplay_DuplicateBeforeExpiryIsReplay(t *testing.T) {
	r := openReplay(t)
	exp := time.Now().Add(time.Minute)
	if err := r.Witness(context.Background(), "jti-2", exp); err != nil {
		t.Fatalf("Witness 1: %v", err)
	}
	err := r.Witness(context.Background(), "jti-2", exp)
	if !errors.Is(err, replay.ErrReplay) {
		t.Errorf("Witness 2: %v, want ErrReplay", err)
	}
}

func TestJWTReplay_DuplicateAfterExpiryIsAcceptedAndRefreshed(t *testing.T) {
	r := openReplay(t)
	past := time.Now().Add(-time.Minute)
	if err := r.Witness(context.Background(), "jti-3", past); err != nil {
		t.Fatalf("Witness 1: %v", err)
	}
	if err := r.Witness(context.Background(), "jti-3", time.Now().Add(time.Minute)); err != nil {
		t.Errorf("Witness 2 after stale: %v", err)
	}
}
