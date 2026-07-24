// Package stepca implements the SUP §3 "Registration Authority Mode" issuer
// backend that delegates X.509-SVID signing to a Step CA instance over its
// /1.0/sign admin API. MIS authenticates via a JWK provisioner token (OTT).
package stepca

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// IssuerConfig parametrises NewIssuer.
type IssuerConfig struct {
	TrustDomain string
	Client      *Client
}

// Issuer delegates SVID signing to Step CA over /1.0/sign. Implements
// identity.Issuer.
type Issuer struct {
	trustDomain string
	client      *Client
}

// NewIssuer constructs an Issuer.
func NewIssuer(cfg IssuerConfig) (*Issuer, error) {
	if cfg.TrustDomain == "" {
		return nil, errors.New("stepca: trust_domain is required")
	}
	if cfg.Client == nil {
		return nil, errors.New("stepca: client is required")
	}
	return &Issuer{trustDomain: cfg.TrustDomain, client: cfg.Client}, nil
}

// IssueX509SVID implements identity.Issuer. Enforces the same policy as
// pkg/mis/ca (ECDSA P-256 only, RSA >= 3072, SPIFFE ID must be in trust
// domain) so both backends reject the same set of invalid inputs. Error
// ordering differs slightly: this backend validates the SPIFFE ID first
// (cheap string check) then the CSR. Every CSR field except the public key
// is ignored - the issued cert's URI SAN is set by Step CA from the OTT
// user.sans claim MIS minted.
func (i *Issuer) IssueX509SVID(ctx context.Context, spiffeID string, csrDER []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	if err := ctx.Err(); err != nil {
		return nil, time.Time{}, err
	}

	uri, err := url.Parse(spiffeID)
	if err != nil || uri.Scheme != "spiffe" {
		return nil, time.Time{}, fmt.Errorf("stepca: invalid SPIFFE ID %q", spiffeID)
	}
	if uri.Host != i.trustDomain {
		return nil, time.Time{}, fmt.Errorf("stepca: SPIFFE ID %q is not in trust domain %q", spiffeID, i.trustDomain)
	}

	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: parse CSR: %w", err)
	}
	if err := supportedPublicKey(csr.PublicKey); err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: CSR public key rejected: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: CSR signature invalid: %w", err)
	}

	chain, notAfter, err := i.client.Sign(ctx, SignRequest{
		SpiffeID: spiffeID,
		CSRDER:   csrDER,
		TTL:      ttl,
	})
	if err != nil {
		return nil, time.Time{}, err
	}
	return chain, notAfter, nil
}

// supportedPublicKey mirrors pkg/mis/ca's policy. ECDSA P-256 only; RSA >= 3072.
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
