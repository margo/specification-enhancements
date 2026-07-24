package jwtexchange_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/margo/miaf-poc/pkg/mis/jwtexchange"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

// testCA mints a self-signed root and signs leaf certificates.
type testCA struct {
	rootKey  *ecdsa.PrivateKey
	rootCert *x509.Certificate
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("root keygen: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("root sign: %v", err)
	}
	root, _ := x509.ParseCertificate(der)
	return &testCA{rootKey: key, rootCert: root}
}

func (c *testCA) pool() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(c.rootCert)
	return p
}

// issueLeaf signs a leaf certificate with the CA's root key. The
// SignatureAlgorithm is left zero so x509.CreateCertificate picks one that
// matches the CA's ECDSA root key.
func (c *testCA) issueLeaf(t *testing.T, spiffeID string, pub crypto.PublicKey) *x509.Certificate {
	t.Helper()
	u, _ := url.Parse(spiffeID)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, c.rootCert, pub, c.rootKey)
	if err != nil {
		t.Fatalf("leaf sign: %v", err)
	}
	leaf, _ := x509.ParseCertificate(der)
	return leaf
}

func makeECDSALeaf(t *testing.T, ca *testCA, spiffeID string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf keygen: %v", err)
	}
	leaf := ca.issueLeaf(t, spiffeID, &key.PublicKey)
	return leaf, key
}

// chainHeader builds a JOSE x5c header value from a chain.
func chainHeader(chain []*x509.Certificate) []string {
	out := make([]string, len(chain))
	for i, c := range chain {
		out[i] = base64.StdEncoding.EncodeToString(c.Raw)
	}
	return out
}

// mkAssertionWithChain mints a signed JWT with the given chain in x5c.
func mkAssertionWithChain(t *testing.T, key crypto.Signer, chain []*x509.Certificate, claims jwt.Claims, alg jose.SignatureAlgorithm) string {
	t.Helper()
	sopts := (&jose.SignerOptions{}).
		WithType("JWT").
		WithHeader(jose.HeaderKey("x5c"), chainHeader(chain))
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key}, sopts)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	out, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	return out
}

// mkAssertionNoX5c mints a signed JWT WITHOUT an x5c header (for negative tests).
func mkAssertionNoX5c(t *testing.T, key crypto.Signer, claims jwt.Claims, alg jose.SignatureAlgorithm) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: alg, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	out, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	return out
}

func openReplay(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mis.sqlite3"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestVerifyClientAssertion_HappyPath(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	const audURL = "https://mis/api/v1/identities/abc/jwt-svid"

	ca := newTestCA(t)
	leaf, key := makeECDSALeaf(t, ca, spiffeID)
	chain := []*x509.Certificate{leaf, ca.rootCert}

	now := time.Now().UTC().Truncate(time.Second)
	assertion := mkAssertionWithChain(t, key, chain, jwt.Claims{
		Issuer:   spiffeID,
		Subject:  spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)

	s := openReplay(t)
	out, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		Roots:       ca.pool(),
		Replay:      s.JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: audURL,
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}, spiffeID, assertion)
	if err != nil {
		t.Fatalf("VerifyClientAssertion: %v", err)
	}
	if out.SPIFFEID != spiffeID {
		t.Errorf("SPIFFEID = %q, want %q", out.SPIFFEID, spiffeID)
	}
}

func TestVerifyClientAssertion_MissingX5c(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	const audURL = "https://aud/"

	ca := newTestCA(t)
	_, key := makeECDSALeaf(t, ca, spiffeID)

	now := time.Now().UTC()
	assertion := mkAssertionNoX5c(t, key, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)

	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		Roots:       ca.pool(),
		Replay:      openReplay(t).JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: audURL,
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}, spiffeID, assertion)
	if !errors.Is(err, jwtexchange.ErrAssertionInvalid) {
		t.Errorf("err = %v, want ErrAssertionInvalid", err)
	}
}

func TestVerifyClientAssertion_UntrustedChain(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	const audURL = "https://aud/"

	// Sign the leaf with caA but verify against caB's pool.
	caA := newTestCA(t)
	caB := newTestCA(t)
	leaf, key := makeECDSALeaf(t, caA, spiffeID)
	chain := []*x509.Certificate{leaf, caA.rootCert}

	now := time.Now().UTC()
	assertion := mkAssertionWithChain(t, key, chain, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)

	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		Roots:       caB.pool(),
		Replay:      openReplay(t).JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: audURL,
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}, spiffeID, assertion)
	if !errors.Is(err, jwtexchange.ErrUntrustedChain) {
		t.Errorf("err = %v, want ErrUntrustedChain", err)
	}
}

func TestVerifyClientAssertion_LeafSPIFFEIDMismatch(t *testing.T) {
	const certSPIFFEID = "spiffe://td/margo/device/cert"
	const pathSPIFFEID = "spiffe://td/margo/device/other"
	const audURL = "https://aud/"

	ca := newTestCA(t)
	leaf, key := makeECDSALeaf(t, ca, certSPIFFEID)
	chain := []*x509.Certificate{leaf, ca.rootCert}

	now := time.Now().UTC()
	assertion := mkAssertionWithChain(t, key, chain, jwt.Claims{
		Issuer: pathSPIFFEID, Subject: pathSPIFFEID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)

	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		Roots:       ca.pool(),
		Replay:      openReplay(t).JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: audURL,
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}, pathSPIFFEID, assertion)
	if !errors.Is(err, jwtexchange.ErrUnknownSPIFFEID) {
		t.Errorf("err = %v, want ErrUnknownSPIFFEID", err)
	}
}

func TestVerifyClientAssertion_AudMismatch(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	ca := newTestCA(t)
	leaf, key := makeECDSALeaf(t, ca, spiffeID)
	chain := []*x509.Certificate{leaf, ca.rootCert}

	now := time.Now().UTC()
	assertion := mkAssertionWithChain(t, key, chain, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{"https://wrong/"},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)
	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		Roots:       ca.pool(),
		Replay:      openReplay(t).JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: "https://right/",
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}, spiffeID, assertion)
	if !errors.Is(err, jwtexchange.ErrAssertionInvalid) {
		t.Errorf("err = %v, want ErrAssertionInvalid", err)
	}
}

func TestVerifyClientAssertion_LifetimeTooLong(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	const audURL = "https://aud/"
	ca := newTestCA(t)
	leaf, key := makeECDSALeaf(t, ca, spiffeID)
	chain := []*x509.Certificate{leaf, ca.rootCert}

	now := time.Now().UTC()
	assertion := mkAssertionWithChain(t, key, chain, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(6 * time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)
	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		Roots:       ca.pool(),
		Replay:      openReplay(t).JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: audURL,
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}, spiffeID, assertion)
	if !errors.Is(err, jwtexchange.ErrAssertionInvalid) {
		t.Errorf("err = %v, want ErrAssertionInvalid", err)
	}
}

// makeRSALeaf builds an RSA leaf certificate against the given CA.
func makeRSALeaf(t *testing.T, ca *testCA, spiffeID string) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	if testing.Short() {
		t.Skip("RSA 3072 keygen is slow; -short skips")
	}
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	leaf := ca.issueLeaf(t, spiffeID, &key.PublicKey)
	return leaf, key
}

func TestVerifyClientAssertion_AlgKeyTypeMismatch(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	const audURL = "https://aud/"

	// Cert chain has an RSA leaf, but the assertion is signed with ES256.
	ca := newTestCA(t)
	rsaLeaf, _ := makeRSALeaf(t, ca, spiffeID)
	chain := []*x509.Certificate{rsaLeaf, ca.rootCert}
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	now := time.Now().UTC()
	assertion := mkAssertionWithChain(t, ecKey, chain, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)

	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		Roots:       ca.pool(),
		Replay:      openReplay(t).JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: audURL,
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}, spiffeID, assertion)
	if !errors.Is(err, jwtexchange.ErrUnsupportedAlg) {
		t.Errorf("err = %v, want ErrUnsupportedAlg", err)
	}
}

func TestVerifyClientAssertion_Replay(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	const audURL = "https://aud/"
	ca := newTestCA(t)
	leaf, key := makeECDSALeaf(t, ca, spiffeID)
	chain := []*x509.Certificate{leaf, ca.rootCert}
	now := time.Now().UTC()

	jti := uuid.NewString()
	make := func() string {
		return mkAssertionWithChain(t, key, chain, jwt.Claims{
			Issuer: spiffeID, Subject: spiffeID,
			Audience: jwt.Audience{audURL},
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
			ID:       jti,
		}, jose.ES256)
	}

	s := openReplay(t)
	cfg := jwtexchange.VerifyConfig{
		Roots:       ca.pool(),
		Replay:      s.JWTReplay(),
		Now:         func() time.Time { return now },
		ExpectedAud: audURL,
		MaxLifetime: 5 * time.Minute,
		ClockSkew:   30 * time.Second,
	}
	if _, err := jwtexchange.VerifyClientAssertion(context.Background(), cfg, spiffeID, make()); err != nil {
		t.Fatalf("first call: %v", err)
	}
	_, err := jwtexchange.VerifyClientAssertion(context.Background(), cfg, spiffeID, make())
	if !errors.Is(err, jwtexchange.ErrReplay) {
		t.Errorf("second call: err = %v, want ErrReplay", err)
	}
}
