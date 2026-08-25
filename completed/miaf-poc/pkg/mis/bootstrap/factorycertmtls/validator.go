// Package factorycertmtls implements the factory-certificate-mTLS bootstrap
// method defined in SUP Appendix A.
package factorycertmtls

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
)

// Error sentinels mapped to SUP Appendix B rows by the enrollment handler.
var (
	ErrMissingMTLS     = errors.New("factory-cert-mtls: no TLS peer certificate presented")
	ErrUntrustedChain  = errors.New("factory-cert-mtls: presented chain does not verify against factory trust pool")
	ErrUnexpectedProof = errors.New("factory-cert-mtls: bootstrapCredential.proof must be omitted")
)

// Validator verifies factory mTLS proofs and derives the ESI.
type Validator struct {
	roots *x509.CertPool
}

// New constructs a Validator with the factory root pool. The pool MUST
// contain the root (and any intermediate) CAs trusted for factory device
// enrollment.
func New(roots *x509.CertPool) *Validator {
	return &Validator{roots: roots}
}

// NewFromPEMFile loads a PEM bundle of factory root CAs and returns a
// Validator over them.
func NewFromPEMFile(path string) (*Validator, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read factory-ca-file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(b) {
		return nil, fmt.Errorf("no PEM certificates in %s", path)
	}
	return New(pool), nil
}

// Validate implements bootstrap.Validator for factory-cert-mtls.
func (v *Validator) Validate(_ context.Context, r *http.Request, cred common.BootstrapCredentialDTO, _, _ string) (bootstrap.ValidationOutcome, error) {
	if cred.Proof != nil && (cred.Proof.Assertion != "" || cred.Proof.Token != "") {
		return bootstrap.ValidationOutcome{}, ErrUnexpectedProof
	}

	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return bootstrap.ValidationOutcome{}, ErrMissingMTLS
	}

	leaf := r.TLS.PeerCertificates[0]
	intermediates := x509.NewCertPool()
	for _, c := range r.TLS.PeerCertificates[1:] {
		intermediates.AddCert(c)
	}
	opts := x509.VerifyOptions{
		Roots:         v.roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if _, err := leaf.Verify(opts); err != nil {
		return bootstrap.ValidationOutcome{}, fmt.Errorf("%w: %v", ErrUntrustedChain, err)
	}

	sum := sha256.Sum256(leaf.Raw)
	return bootstrap.ValidationOutcome{
		Pending: &bootstrap.PendingEnrollment{
			VB: bootstrap.VerifiedBootstrap{
				Method:   cred.Method,
				ESI:      sum[:],
				Evidence: leaf,
			},
		},
	}, nil
}
