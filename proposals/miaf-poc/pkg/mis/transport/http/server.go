package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Config bundles dependencies required to construct a Server.
type Config struct {
	Logger *slog.Logger
	// DiscoveryHandler handles GET /.well-known/margo.
	DiscoveryHandler http.Handler
	// BundleMapHandler handles GET /.well-known/spiffe/bundle.json.
	BundleMapHandler http.Handler
	// EnrollmentHandler handles POST /api/v1/identities.
	EnrollmentHandler http.Handler
	// RenewalHandler handles POST /api/v1/identities/{spiffeIdEncoded}/renewal.
	RenewalHandler http.Handler
	// JWTSVIDHandler handles POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid.
	JWTSVIDHandler http.Handler
	// RevocationsHandler handles GET /api/v1/revocations.
	RevocationsHandler http.Handler
	// ClientCAPool is the set of root CAs the MIS advertises in the TLS
	// CertificateRequest. It must cover both the factory root CAs (for
	// factory-cert-mtls enrollment) and the MIS root CA (so Go's TLS client
	// presents the SVID as a client cert during renewal).
	ClientCAPool *x509.CertPool
}

// Server holds registered routes. Use Handler() to obtain the wrapped
// http.Handler suitable for an httptest.Server or a TLS ListenAndServe.
type Server struct {
	cfg     Config
	handler http.Handler
}

// NewServer builds the routes and applies middleware.
func NewServer(cfg Config) *Server {
	if cfg.DiscoveryHandler == nil {
		panic("http: Config.DiscoveryHandler is required")
	}
	if cfg.BundleMapHandler == nil {
		panic("http: Config.BundleMapHandler is required")
	}
	if cfg.EnrollmentHandler == nil {
		panic("http: Config.EnrollmentHandler is required")
	}
	if cfg.RenewalHandler == nil {
		panic("http: Config.RenewalHandler is required")
	}
	if cfg.JWTSVIDHandler == nil {
		panic("http: Config.JWTSVIDHandler is required")
	}
	if cfg.RevocationsHandler == nil {
		panic("http: Config.RevocationsHandler is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.Handle("GET /.well-known/margo", cfg.DiscoveryHandler)
	mux.Handle("GET /.well-known/spiffe/bundle.json", cfg.BundleMapHandler)
	mux.Handle("POST /api/v1/identities", cfg.EnrollmentHandler)
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/renewal", cfg.RenewalHandler)
	mux.Handle("POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid", cfg.JWTSVIDHandler)
	mux.Handle("GET /api/v1/revocations", cfg.RevocationsHandler)

	handler := http.Handler(mux)
	handler = AccessLog(cfg.Logger)(handler)
	handler = WithRequestID(handler)

	return &Server{cfg: cfg, handler: handler}
}

// Handler returns the wrapped http.Handler.
func (s *Server) Handler() http.Handler { return s.handler }

// ListenAndServeTLS runs the MIS HTTPS server until ctx is cancelled or the
// listener fails. The server enforces TLS 1.3 (SUP §7 "Minimum TLS Baseline")
// and, when cfg.ClientCAPool is set, requests (optional) client certs so
// factory-cert-mtls enrollment and SVID-mTLS renewal can coexist with
// anonymous HTTPS for the discovery and bundle-map endpoints.
func (s *Server) ListenAndServeTLS(ctx context.Context, addr, certFile, keyFile string) error {
	serverCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load server TLS keypair: %w", err)
	}
	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,
	}
	if s.cfg.ClientCAPool != nil {
		tlsCfg.ClientCAs = s.cfg.ClientCAPool
		tlsCfg.ClientAuth = tls.VerifyClientCertIfGiven
	}

	hs := &http.Server{
		Addr:              addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 5 * time.Second,
		TLSConfig:         tlsCfg,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- hs.ListenAndServeTLS("", "") }()

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
