// Package ca implements MIS's in-process Intermediate CA issuer. Device
// X.509-SVIDs are minted by calling crypto/x509.CreateCertificate against an
// operator-provisioned intermediate CA keypair loaded from disk. This is
// SUP §3 "Intermediate CA Mode".
package ca

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"sync/atomic"
	"time"
)

// Config parametrises NewIntermediateIssuer.
type Config struct {
	// TrustDomain is the MIS's local trust domain (e.g. "margo.example.com").
	// The intermediate cert's URI SAN and every issued leaf's URI SAN must
	// reside within this trust domain.
	TrustDomain string
	// CertFile is a PEM file containing the intermediate CA certificate.
	CertFile string
	// KeyFile is a PEM file containing the intermediate CA private key (PKCS#8).
	KeyFile string
}

// IntermediateIssuer is an implementation of identity.Issuer that signs device
// SVIDs with an intermediate CA. The intermediate is presented on the chain
// returned from IssueX509SVID so clients can validate up to the root anchor
// in the Bundle Map.
type IntermediateIssuer struct {
	trustDomain   string
	intermediate  *x509.Certificate
	key           crypto.Signer
	serialCounter *serialGenerator
}

// NewIntermediateIssuer loads the intermediate cert and key from disk and
// returns a ready-to-use issuer. Validates that the loaded intermediate is a
// CA cert whose URI SAN matches the supplied trust domain.
func NewIntermediateIssuer(cfg Config) (*IntermediateIssuer, error) {
	if cfg.TrustDomain == "" {
		return nil, errors.New("ca: trust_domain is required")
	}
	cert, err := loadCert(cfg.CertFile)
	if err != nil {
		return nil, fmt.Errorf("ca: load intermediate cert: %w", err)
	}
	if !cert.IsCA {
		return nil, errors.New("ca: intermediate cert is not a CA (IsCA=false)")
	}
	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, errors.New("ca: intermediate cert missing KeyUsage=certSign")
	}
	if err := supportedPublicKey(cert.PublicKey); err != nil {
		return nil, fmt.Errorf("ca: intermediate cert public key rejected: %w", err)
	}
	if !matchesTrustDomainAuthority(cert, cfg.TrustDomain) {
		return nil, fmt.Errorf("ca: intermediate cert URI SANs = %v, want exactly [spiffe://%s/] (path-less, per SPIFFE X509-SVID §3.2)", cert.URIs, cfg.TrustDomain)
	}
	key, err := loadKey(cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("ca: load intermediate key: %w", err)
	}
	if err := matchPubKey(cert, key); err != nil {
		return nil, fmt.Errorf("ca: intermediate cert/key mismatch: %w", err)
	}
	return &IntermediateIssuer{
		trustDomain:   cfg.TrustDomain,
		intermediate:  cert,
		key:           key,
		serialCounter: &serialGenerator{},
	}, nil
}

// IssueX509SVID implements identity.Issuer. It ignores every field of the
// parsed CSR except the public key, per SUP §5.
func (i *IntermediateIssuer) IssueX509SVID(ctx context.Context, spiffeID string, csrDER []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("ca: parse CSR: %w", err)
	}
	// Reject unsupported key types before the expensive signature check.
	if err := supportedPublicKey(csr.PublicKey); err != nil {
		return nil, time.Time{}, fmt.Errorf("ca: CSR public key rejected: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, time.Time{}, fmt.Errorf("ca: CSR signature invalid: %w", err)
	}

	uri, err := url.Parse(spiffeID)
	if err != nil || uri.Scheme != "spiffe" {
		return nil, time.Time{}, fmt.Errorf("ca: invalid SPIFFE ID %q", spiffeID)
	}
	if uri.Host != i.trustDomain {
		return nil, time.Time{}, fmt.Errorf("ca: SPIFFE ID %q is not in trust domain %q", spiffeID, i.trustDomain)
	}

	now := time.Now().UTC()
	notAfter := now.Add(ttl)
	if notAfter.After(i.intermediate.NotAfter) {
		notAfter = i.intermediate.NotAfter
	}

	leafTmpl := &x509.Certificate{
		SerialNumber:          i.serialCounter.Next(),
		Subject:               pkix.Name{}, // explicitly empty; SPIFFE IDs live in the URI SAN
		NotBefore:             now.Add(-30 * time.Second),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		URIs:                  []*url.URL{uri},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, i.intermediate, csr.PublicKey, i.key)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("ca: sign leaf: %w", err)
	}
	return [][]byte{leafDER, i.intermediate.Raw}, notAfter, nil
}

// supportedPublicKey enforces the SUP §3 "Cryptographic Requirements" table.
// ECDSA P-256 only; RSA >= 3072. Everything else is rejected.
func supportedPublicKey(pub any) error {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		if k.Curve != elliptic.P256() {
			return fmt.Errorf("unsupported EC curve %q", k.Curve.Params().Name)
		}
		return nil
	case *rsa.PublicKey:
		if k.N.BitLen() < 3072 {
			return fmt.Errorf("RSA key too small: %d bits (minimum 3072)", k.N.BitLen())
		}
		return nil
	default:
		return fmt.Errorf("unsupported public key type %T", pub)
	}
}

func loadCert(path string) (*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("no CERTIFICATE PEM block in %s", path)
	}
	if next, _ := pem.Decode(rest); next != nil && next.Type == "CERTIFICATE" {
		return nil, fmt.Errorf("intermediate cert file %s contains multiple certificates; supply only the leaf cert", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

func loadKey(path string) (crypto.Signer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	// Try PKCS#8 first (what `step` emits), then SEC1 EC, then PKCS#1 RSA.
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		s, ok := k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("key in %s is not a crypto.Signer", path)
		}
		return s, nil
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, fmt.Errorf("unsupported key format in %s", path)
}

func matchPubKey(cert *x509.Certificate, key crypto.Signer) error {
	switch p := cert.PublicKey.(type) {
	case *ecdsa.PublicKey:
		k, ok := key.(*ecdsa.PrivateKey)
		if !ok || !k.PublicKey.Equal(p) {
			return errors.New("ECDSA public/private key mismatch")
		}
	case *rsa.PublicKey:
		k, ok := key.(*rsa.PrivateKey)
		if !ok || !k.PublicKey.Equal(p) {
			return errors.New("RSA public/private key mismatch")
		}
	default:
		return fmt.Errorf("unsupported key type %T", p)
	}
	return nil
}

// matchesTrustDomainAuthority returns true iff cert has exactly one URI SAN
// that is an authority-only SPIFFE ID (empty or root-only path) in the given
// trust domain. Per SPIFFE X509-SVID §3.2, signing certs MUST NOT have a path
// component. Both "spiffe://<td>" and "spiffe://<td>/" are accepted as the
// same canonical authority-only form.
func matchesTrustDomainAuthority(cert *x509.Certificate, trustDomain string) bool {
	if len(cert.URIs) != 1 {
		return false
	}
	u := cert.URIs[0]
	if u.Scheme != "spiffe" || u.Host != trustDomain {
		return false
	}
	return u.Path == "" || u.Path == "/"
}

// serialGenerator produces unique serials combining the process start time
// (nanoseconds) with an atomic counter. The time prefix prevents collisions
// across process restarts in the PoC - important because CAs MUST NOT reuse
// a serial for a given issuer. Production should use crypto/rand.Int over a
// 20-byte range (RFC 5280 §4.1.2.2 allows up to 20 octets).
type serialGenerator struct {
	base atomic.Int64
	n    atomic.Int64
}

func (s *serialGenerator) Next() *big.Int {
	if s.base.Load() == 0 {
		s.base.CompareAndSwap(0, time.Now().UnixNano())
	}
	n := s.n.Add(1)
	out := big.NewInt(s.base.Load())
	out.Lsh(out, 20)
	out.Add(out, big.NewInt(n))
	return out
}
