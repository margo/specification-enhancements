// Package httpclient builds the HTTPS client the device-agent uses to reach
// MIS. The trust anchor is pinned at construction - this mirrors the "initial
// trust bootstrap" clause in SUP §7.
package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"time"
)

// New returns a client that only trusts servers presenting a certificate
// chain verifiable against the PEM bundle at trustAnchorFile.
func New(trustAnchorFile string) (*http.Client, error) {
	pool, err := loadPEMPool(trustAnchorFile)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    pool,
				MinVersion: tls.VersionTLS13,
			},
		},
	}, nil
}

// NewMTLS is like New, plus attaches a client certificate + key for mutual
// TLS.
func NewMTLS(trustAnchorFile, clientCertFile, clientKeyFile string) (*http.Client, error) {
	pool, err := loadPEMPool(trustAnchorFile)
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client keypair: %w", err)
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:      pool,
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS13,
			},
		},
	}, nil
}

// loadPEMPool returns nil (system trust store) when path is empty.
func loadPEMPool(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read trust anchor: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("no PEM certificates in %s", path)
	}
	return pool, nil
}
