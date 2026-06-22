package ca

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

// TrustAnchorsConfig parametrises NewTrustAnchors.
type TrustAnchorsConfig struct {
	// TrustDomain is the local trust domain the bundle represents.
	TrustDomain string
	// RootFile is a PEM file containing one or more X.509 root CA certificates.
	// Multiple concatenated PEM blocks are supported (root-rotation scenario).
	RootFile string
	// JWTAuthorities is published verbatim under the trust domain entry's
	// `keys[]` array with `use="jwt-svid"`.
	JWTAuthorities []bundlemap.JWTAuthority
}

// TrustAnchors is an in-memory snapshot of the trust-anchor set loaded at
// startup. Implements transport/http.BundleSource.
// mu guards set and loadedAt; a future reload path must replace the set
// atomically under a write lock (never mutate the X509Authorities slice
// in place - replace the whole TrustAnchorSet reference).
type TrustAnchors struct {
	mu       sync.RWMutex
	set      bundlemap.TrustAnchorSet
	loadedAt time.Time
}

// Compile-time assertion: *TrustAnchors must satisfy mishttp.BundleSource.
var _ mishttp.BundleSource = (*TrustAnchors)(nil)

// NewTrustAnchors reads the root certificate file and returns a TrustAnchors
// snapshot. Errors if the file is missing or has no valid CERTIFICATE PEM.
func NewTrustAnchors(cfg TrustAnchorsConfig) (*TrustAnchors, error) {
	if cfg.TrustDomain == "" {
		return nil, errors.New("trustanchors: trust_domain is required")
	}
	if cfg.RootFile == "" {
		return nil, errors.New("trustanchors: root_file is required")
	}
	raw, err := os.ReadFile(cfg.RootFile)
	if err != nil {
		return nil, fmt.Errorf("trustanchors: read %s: %w", cfg.RootFile, err)
	}

	var certs []*x509.Certificate
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("trustanchors: parse certificate in %s: %w", cfg.RootFile, err)
		}
		certs = append(certs, c)
	}
	if len(certs) == 0 {
		return nil, fmt.Errorf("trustanchors: no CERTIFICATE PEM blocks in %s", cfg.RootFile)
	}

	return &TrustAnchors{
		set: bundlemap.TrustAnchorSet{
			TrustDomain:     cfg.TrustDomain,
			X509Authorities: certs,
			JWTAuthorities:  cfg.JWTAuthorities,
			SequenceNumber:  0,
		},
		loadedAt: time.Now().UTC(),
	}, nil
}

// GetTrustAnchors implements transport/http.BundleSource.
func (a *TrustAnchors) GetTrustAnchors(ctx context.Context) (bundlemap.TrustAnchorSet, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return bundlemap.TrustAnchorSet{}, time.Time{}, err
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.set, a.loadedAt, nil
}
