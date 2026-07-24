package store_test

import (
	"path/filepath"
	"testing"

	"github.com/margo/miaf-poc/pkg/mis/store"
)

func TestOpen_AppliesSchemaIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mis.sqlite3")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open (first): %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	s, err = store.Open(path)
	if err != nil {
		t.Fatalf("Open (second, existing file): %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestOpen_RejectsInvalidPath(t *testing.T) {
	if _, err := store.Open("/nonexistent-dir/mis.sqlite3"); err == nil {
		t.Fatal("Open should fail for unreachable path")
	}
}
