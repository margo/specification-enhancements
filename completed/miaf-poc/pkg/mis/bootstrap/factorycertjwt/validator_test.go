package factorycertjwt_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
	"github.com/margo/miaf-poc/test/testsupport/bootstrapjwt"
)

const testAudience = "https://mis.example.com/api/v1/identities"

func TestValidate_HappyPath_DerivesESIFromLeafFingerprint(t *testing.T) {
	root, rootKey := mustCA(t, "Test Factory Root")
	leaf, leafKey := mustLeaf(t, root, rootKey, "test-device")

	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots:      pool,
		Audience:   testAudience,
		Replay:     replay.NewMemoryStore(replay.Config{}),
		ClockSkew:  30 * time.Second,
		Clock:      time.Now,
	})

	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{})

	req := httptest.NewRequest(http.MethodPost, testAudience, nil)
	cred := common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodFactoryCertJWT,
		Proof:  &common.BootstrapProof{Assertion: a.Compact},
	}
	outcome, err := v.Validate(context.Background(), req, cred, "", "")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if outcome.Pending == nil {
		t.Fatal("outcome.Pending is nil")
	}
	vb := outcome.Pending.VB
	want := sha256.Sum256(leaf.Raw)
	if !bytes.Equal(vb.ESI, want[:]) {
		t.Errorf("ESI mismatch\ngot:  %x\nwant: %x", vb.ESI, want[:])
	}
	if vb.Method != common.BootstrapMethodFactoryCertJWT {
		t.Errorf("Method = %q", vb.Method)
	}
	if vb.Evidence == nil {
		t.Error("Evidence must carry audit material (validated leaf)")
	}
}

func TestValidate_UntrustedChain_ReturnsErrUntrustedChain(t *testing.T) {
	trusted, _ := mustCA(t, "Trusted")
	rogue, rogueKey := mustCA(t, "Rogue")
	leaf, leafKey := mustLeaf(t, rogue, rogueKey, "rogue-device")

	pool := x509.NewCertPool()
	pool.AddCert(trusted)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})
	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, rogue}, leafKey, testAudience, bootstrapjwt.Options{})

	req := httptest.NewRequest(http.MethodPost, testAudience, nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodFactoryCertJWT,
		Proof:  &common.BootstrapProof{Assertion: a.Compact},
	}, "", "")
	if !errors.Is(err, factorycertjwt.ErrUntrustedChain) {
		t.Errorf("err = %v, want ErrUntrustedChain", err)
	}
}

func TestValidate_MissingProof_ReturnsErrMissingAssertion(t *testing.T) {
	pool := x509.NewCertPool()
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})
	req := httptest.NewRequest(http.MethodPost, testAudience, nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodFactoryCertJWT,
	}, "", "")
	if !errors.Is(err, factorycertjwt.ErrMissingAssertion) {
		t.Errorf("err = %v, want ErrMissingAssertion", err)
	}
}

func TestValidate_BadSignature_ReturnsErrBadSignature(t *testing.T) {
	root, rootKey := mustCA(t, "Root")
	leaf, leafKey := mustLeaf(t, root, rootKey, "dev")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})

	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{})
	tampered := a.Compact[:len(a.Compact)-4] + "AAAA" // mangle trailing signature bytes

	req := httptest.NewRequest(http.MethodPost, testAudience, nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodFactoryCertJWT,
		Proof:  &common.BootstrapProof{Assertion: tampered},
	}, "", "")
	if !errors.Is(err, factorycertjwt.ErrBadSignature) {
		t.Errorf("err = %v, want ErrBadSignature", err)
	}
}

func TestValidate_MissingX5C_ReturnsErrMalformedAssertion(t *testing.T) {
	root, rootKey := mustCA(t, "Root")
	leaf, leafKey := mustLeaf(t, root, rootKey, "dev")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})

	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{SkipX5C: true})

	req := httptest.NewRequest(http.MethodPost, testAudience, nil)
	_, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
		Method: common.BootstrapMethodFactoryCertJWT,
		Proof:  &common.BootstrapProof{Assertion: a.Compact},
	}, "", "")
	if !errors.Is(err, factorycertjwt.ErrMalformedAssertion) {
		t.Errorf("err = %v, want ErrMalformedAssertion", err)
	}
}

func TestValidate_AudienceMismatch_ReturnsErrClaimInvalid(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})

	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, "https://other.example.com/api/v1/identities", bootstrapjwt.Options{})
	_, err := v.Validate(context.Background(), httptest.NewRequest(http.MethodPost, testAudience, nil),
		common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertJWT, Proof: &common.BootstrapProof{Assertion: a.Compact}}, "", "")
	if !errors.Is(err, factorycertjwt.ErrClaimInvalid) {
		t.Errorf("err = %v, want ErrClaimInvalid", err)
	}
}

func TestValidate_Expired_ReturnsErrClaimInvalid(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)

	fixed := time.Unix(1_000_000_000, 0)
	clock := func() time.Time { return fixed }
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{Clock: clock}),
		Clock:  clock,
	})
	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{
		ClaimsOverride: func(c *bootstrapjwt.Claims) {
			c.IssuedAt = fixed.Add(-10 * time.Minute)
			c.Expiry = fixed.Add(-1 * time.Minute)
		},
	})
	_, err := v.Validate(context.Background(), httptest.NewRequest(http.MethodPost, testAudience, nil),
		common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertJWT, Proof: &common.BootstrapProof{Assertion: a.Compact}}, "", "")
	if !errors.Is(err, factorycertjwt.ErrClaimInvalid) {
		t.Errorf("err = %v, want ErrClaimInvalid", err)
	}
}

func TestValidate_NotYetValid_ReturnsErrClaimInvalid(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)

	fixed := time.Unix(1_000_000_000, 0)
	clock := func() time.Time { return fixed }
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{Clock: clock}),
		Clock:  clock,
	})
	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{
		ClaimsOverride: func(c *bootstrapjwt.Claims) {
			c.NotBefore = fixed.Add(10 * time.Minute)
			c.IssuedAt = fixed.Add(10 * time.Minute)
			c.Expiry = fixed.Add(14 * time.Minute)
		},
	})
	_, err := v.Validate(context.Background(), httptest.NewRequest(http.MethodPost, testAudience, nil),
		common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertJWT, Proof: &common.BootstrapProof{Assertion: a.Compact}}, "", "")
	if !errors.Is(err, factorycertjwt.ErrClaimInvalid) {
		t.Errorf("err = %v, want ErrClaimInvalid", err)
	}
}

func TestValidate_LifetimeTooLong_ReturnsErrClaimInvalid(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})
	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{
		ClaimsOverride: func(c *bootstrapjwt.Claims) {
			c.Expiry = c.IssuedAt.Add(10 * time.Minute) // > 300s max
		},
	})
	_, err := v.Validate(context.Background(), httptest.NewRequest(http.MethodPost, testAudience, nil),
		common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertJWT, Proof: &common.BootstrapProof{Assertion: a.Compact}}, "", "")
	if !errors.Is(err, factorycertjwt.ErrClaimInvalid) {
		t.Errorf("err = %v, want ErrClaimInvalid", err)
	}
}

func TestValidate_IssuerMismatch_ReturnsErrClaimInvalid(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})
	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{
		ClaimsOverride: func(c *bootstrapjwt.Claims) {
			c.Issuer = "urn:margo:device:sha256:deadbeef"
			c.Subject = c.Issuer
		},
	})
	_, err := v.Validate(context.Background(), httptest.NewRequest(http.MethodPost, testAudience, nil),
		common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertJWT, Proof: &common.BootstrapProof{Assertion: a.Compact}}, "", "")
	if !errors.Is(err, factorycertjwt.ErrClaimInvalid) {
		t.Errorf("err = %v, want ErrClaimInvalid", err)
	}
}

func TestValidate_SubjectNotEqualIssuer_ReturnsErrClaimInvalid(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})
	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{
		ClaimsOverride: func(c *bootstrapjwt.Claims) {
			c.Subject = "urn:margo:device:sha256:different"
		},
	})
	_, err := v.Validate(context.Background(), httptest.NewRequest(http.MethodPost, testAudience, nil),
		common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertJWT, Proof: &common.BootstrapProof{Assertion: a.Compact}}, "", "")
	if !errors.Is(err, factorycertjwt.ErrClaimInvalid) {
		t.Errorf("err = %v, want ErrClaimInvalid", err)
	}
}

func TestValidate_Replay_ReturnsErrReplay(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})

	a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{})
	cred := common.BootstrapCredentialDTO{Method: common.BootstrapMethodFactoryCertJWT, Proof: &common.BootstrapProof{Assertion: a.Compact}}
	req := httptest.NewRequest(http.MethodPost, testAudience, nil)

	if _, err := v.Validate(context.Background(), req, cred, "", ""); err != nil {
		t.Fatalf("first Validate: %v", err)
	}
	_, err := v.Validate(context.Background(), req, cred, "", "")
	if !errors.Is(err, factorycertjwt.ErrReplay) {
		t.Errorf("replay Validate err = %v, want ErrReplay", err)
	}
}

func TestValidate_FreshJTIOnSameDevice_IsAccepted(t *testing.T) {
	root, rootKey := mustCA(t, "R")
	leaf, leafKey := mustLeaf(t, root, rootKey, "d")
	pool := x509.NewCertPool()
	pool.AddCert(root)
	v := factorycertjwt.New(factorycertjwt.Config{
		Roots: pool, Audience: testAudience,
		Replay: replay.NewMemoryStore(replay.Config{}),
	})
	req := httptest.NewRequest(http.MethodPost, testAudience, nil)

	for i := 0; i < 3; i++ {
		a := bootstrapjwt.Mint(t, leaf, []*x509.Certificate{leaf, root}, leafKey, testAudience, bootstrapjwt.Options{})
		if _, err := v.Validate(context.Background(), req, common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertJWT,
			Proof:  &common.BootstrapProof{Assertion: a.Compact},
		}, "", ""); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}
}

// --- helpers -------------------------------------------------------------

func mustCA(t *testing.T, cn string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	cert, _ := x509.ParseCertificate(der)
	return cert, k
}

func mustLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, ca, &k.PublicKey, caKey)
	cert, _ := x509.ParseCertificate(der)
	return cert, k
}
