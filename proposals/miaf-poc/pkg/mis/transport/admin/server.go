// Package admin implements MIS's non-normative administration HTTP API.
//
// The routes in this package are NOT part of the Margo Identity & Authorisation
// Framework SUP. They exist so the PoC can provision operator-issued credentials
// (enrollment tokens, replacement tickets) without direct filesystem access to
// MIS's SQLite database. Production deployments are expected to replace or extend this
// surface; any MIS implementation may omit it entirely without loss of
// MIAF conformance.
//
// This package is intentionally isolated from pkg/mis/transport/http
// (which serves the MIAF surface) - separate package, separate
// listener, separate URL prefix (/admin/v1/...), separate bind address.
package admin

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken/tokens"
	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// RevocationOperator is the admin-side slice of the identity service used by
// POST /admin/v1/revocations. Narrow interface so tests can substitute a fake.
type RevocationOperator interface {
	RevokeSPIFFEID(ctx context.Context, spiffeID, reason string) (newlyRevoked []identity.RevocationEntry, alreadyRevoked []string, err error)
}

// Config bundles dependencies required to construct a Server.
type Config struct {
	EnrollmentTokens   tokens.Store
	ReplacementTickets identity.ReplacementTicketStore
	// Revocations is the operator-revocation port. Typically satisfied by identity.Service.
	Revocations RevocationOperator
	// DefaultTokenTTL is applied when a CreateEnrollmentTokenRequest
	// omits TTL. Zero means TTL is required in every request.
	DefaultTokenTTL time.Duration
	// Logger defaults to slog.Default() when nil.
	Logger *slog.Logger
}

// Server routes /admin/v1/... requests.
type Server struct {
	cfg     Config
	handler http.Handler
}

// NewServer builds a Server with the configured routes.
func NewServer(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	mux := http.NewServeMux()
	s := &Server{cfg: cfg}
	mux.Handle("POST /admin/v1/enrollment-tokens", http.HandlerFunc(s.createEnrollmentToken))
	mux.Handle("POST /admin/v1/replacement-tickets", http.HandlerFunc(s.createReplacementTicket))
	mux.Handle("POST /admin/v1/revocations", http.HandlerFunc(s.createRevocation))
	s.handler = mux
	return s
}

// Handler returns the wrapped http.Handler.
func (s *Server) Handler() http.Handler { return s.handler }

// ListenAndServe binds a plaintext HTTP listener on addr and serves
// the admin routes until ctx is cancelled. Plaintext by design: the
// admin surface is intended for loopback-only binding in the PoC.
func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	hs := &http.Server{
		Addr:              addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- hs.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return hs.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// writeJSON writes v as application/json with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a minimal JSON error body: { "error": "<msg>" }.
// The admin API is explicitly NOT Problem Details - this is an
// internal surface, not MIAF, and should not be confused with the SUP
// error shape served by pkg/mis/transport/http.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
