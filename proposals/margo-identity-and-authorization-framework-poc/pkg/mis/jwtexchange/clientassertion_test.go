package jwtexchange_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"

	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/jwtexchange"
	"github.com/margo/miaf-poc/pkg/mis/store"
)

type stubLeafLookup struct {
	rec identity.IssuedSVIDRecord
	err error
}

func (s *stubLeafLookup) Record(_ context.Context, _ identity.IssuedSVIDRecord) error { return nil }
func (s *stubLeafLookup) ActiveBySPIFFEID(_ context.Context, _ string, _ time.Time) ([]identity.IssuedSVIDRecord, error) {
	return nil, nil
}
func (s *stubLeafLookup) MarkRevoked(_ context.Context, _ string, _ time.Time) error { return nil }
func (s *stubLeafLookup) LeafByActiveSPIFFEID(_ context.Context, _ string, _ time.Time) (identity.IssuedSVIDRecord, error) {
	return s.rec, s.err
}

func makeLeaf(t *testing.T, spiffeID string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u, _ := url.Parse(spiffeID)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	leaf, _ := x509.ParseCertificate(der)
	return leaf, key
}

func mkAssertion(t *testing.T, key *ecdsa.PrivateKey, claims jwt.Claims, alg jose.SignatureAlgorithm) string {
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

	leaf, key := makeLeaf(t, spiffeID)
	now := time.Now().UTC().Truncate(time.Second)
	assertion := mkAssertion(t, key, jwt.Claims{
		Issuer:   spiffeID,
		Subject:  spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)

	s := openReplay(t)
	out, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		LeafLookup:  &stubLeafLookup{rec: identity.IssuedSVIDRecord{LeafDER: leaf.Raw, NotAfter: leaf.NotAfter}},
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

func TestVerifyClientAssertion_AudMismatch(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	leaf, key := makeLeaf(t, spiffeID)
	now := time.Now().UTC()
	assertion := mkAssertion(t, key, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{"https://wrong/"},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)
	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		LeafLookup:  &stubLeafLookup{rec: identity.IssuedSVIDRecord{LeafDER: leaf.Raw, NotAfter: leaf.NotAfter}},
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
	leaf, key := makeLeaf(t, spiffeID)
	now := time.Now().UTC()
	assertion := mkAssertion(t, key, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(6 * time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)
	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		LeafLookup:  &stubLeafLookup{rec: identity.IssuedSVIDRecord{LeafDER: leaf.Raw, NotAfter: leaf.NotAfter}},
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

func makeRSALeaf(t *testing.T, spiffeID string) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	if testing.Short() {
		t.Skip("RSA 3072 keygen is slow; -short skips")
	}
	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	u, _ := url.Parse(spiffeID)
	tmpl := &x509.Certificate{
		SerialNumber:       big.NewInt(43),
		Subject:            pkix.Name{},
		NotBefore:          time.Now().Add(-time.Minute),
		NotAfter:           time.Now().Add(time.Hour),
		URIs:               []*url.URL{u},
		SignatureAlgorithm: x509.SHA256WithRSAPSS,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	leaf, _ := x509.ParseCertificate(der)
	return leaf, key
}

func TestVerifyClientAssertion_AlgKeyTypeMismatch(t *testing.T) {
	const spiffeID = "spiffe://td/margo/device/u1"
	const audURL = "https://aud/"

	// RSA leaf in MIS audit; assertion signed ES256 with a fresh ECDSA key
	rsaLeaf, _ := makeRSALeaf(t, spiffeID)
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	now := time.Now().UTC()
	assertion := mkAssertion(t, ecKey, jwt.Claims{
		Issuer: spiffeID, Subject: spiffeID,
		Audience: jwt.Audience{audURL},
		IssuedAt: jwt.NewNumericDate(now),
		Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
		ID:       uuid.NewString(),
	}, jose.ES256)

	_, err := jwtexchange.VerifyClientAssertion(context.Background(), jwtexchange.VerifyConfig{
		LeafLookup:  &stubLeafLookup{rec: identity.IssuedSVIDRecord{LeafDER: rsaLeaf.Raw, NotAfter: rsaLeaf.NotAfter}},
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
	leaf, key := makeLeaf(t, spiffeID)
	now := time.Now().UTC()

	jti := uuid.NewString()
	make := func() string {
		return mkAssertion(t, key, jwt.Claims{
			Issuer: spiffeID, Subject: spiffeID,
			Audience: jwt.Audience{audURL},
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(time.Minute)),
			ID:       jti,
		}, jose.ES256)
	}

	s := openReplay(t)
	cfg := jwtexchange.VerifyConfig{
		LeafLookup:  &stubLeafLookup{rec: identity.IssuedSVIDRecord{LeafDER: leaf.Raw, NotAfter: leaf.NotAfter}},
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
