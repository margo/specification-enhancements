package identity_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/csr"
	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// --- in-memory Repository stub ---

type memRepo struct {
	mu       sync.Mutex
	ldis     map[string]identity.LDI
	bindings []identity.ESIBinding
	allBound map[string]bool
	nextID   func() string
}

func newMemRepo() *memRepo {
	return &memRepo{
		ldis:     map[string]identity.LDI{},
		allBound: map[string]bool{},
		nextID:   func() string { return "00000000-0000-4000-8000-000000000001" },
	}
}

func (r *memRepo) LookupByESI(ctx context.Context, esi []byte) (identity.ESIBinding, identity.LDI, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// The store-level invariant is "at most one binding row per ESI across
	// active+retired". Return the single match, preferring active so callers
	// see the same ordering the SQL store would yield if an ESI were ever
	// recycled (which the schema forbids).
	var retired *identity.ESIBinding
	for i := range r.bindings {
		b := &r.bindings[i]
		if !bytes.Equal(b.ESI, esi) {
			continue
		}
		if b.Status == identity.ESIBindingActive {
			return *b, r.ldis[b.LDIUUID], nil
		}
		retired = b
	}
	if retired != nil {
		return *retired, r.ldis[retired.LDIUUID], nil
	}
	return identity.ESIBinding{}, identity.LDI{}, identity.ErrNotFound
}

func (r *memRepo) CreateLDIAndBind(ctx context.Context, esi []byte, method, trustDomain string) (identity.LDI, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.allBound[string(esi)] {
		return identity.LDI{}, errors.New("esi already bound")
	}
	id := r.nextID()
	l := identity.LDI{UUID: id, SPIFFEID: "spiffe://" + trustDomain + "/margo/device/" + id, Status: identity.LDIStatusActive, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	r.ldis[id] = l
	r.allBound[string(esi)] = true
	r.bindings = append(r.bindings, identity.ESIBinding{ESI: esi, LDIUUID: id, Method: method, Status: identity.ESIBindingActive, BoundAt: time.Now().UTC()})
	return l, nil
}

func (r *memRepo) Rebind(ctx context.Context, ldiUUID string, esi []byte, method string) (identity.ESIBinding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.ldis[ldiUUID]; !ok {
		return identity.ESIBinding{}, identity.ErrNotFound
	}
	for i := range r.bindings {
		if r.bindings[i].LDIUUID == ldiUUID && r.bindings[i].Status == identity.ESIBindingActive {
			r.bindings[i].Status = identity.ESIBindingRetired
			r.bindings[i].RetiredAt = time.Now().UTC()
		}
	}
	b := identity.ESIBinding{ESI: esi, LDIUUID: ldiUUID, Method: method, Status: identity.ESIBindingActive, BoundAt: time.Now().UTC()}
	r.bindings = append(r.bindings, b)
	r.allBound[string(esi)] = true
	return b, nil
}

func (r *memRepo) GetLDI(ctx context.Context, uuid string) (identity.LDI, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.ldis[uuid]
	if !ok {
		return identity.LDI{}, identity.ErrNotFound
	}
	return l, nil
}

func (r *memRepo) GetLDIBySPIFFEID(ctx context.Context, spiffeID string) (identity.LDI, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.ldis {
		if l.SPIFFEID == spiffeID {
			return l, nil
		}
	}
	return identity.LDI{}, identity.ErrNotFound
}

func (r *memRepo) ListBindings(ctx context.Context, ldiUUID string) ([]identity.ESIBinding, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []identity.ESIBinding
	for _, b := range r.bindings {
		if b.LDIUUID == ldiUUID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *memRepo) SetLDIStatus(ctx context.Context, uuid string, status identity.LDIStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.ldis[uuid]
	if !ok {
		return identity.ErrNotFound
	}
	l.Status = status
	r.ldis[uuid] = l
	return nil
}

// fakeReplacements maps ticket string to ldiUUID; single-use (entry deleted on Validate).
type fakeReplacements map[string]string

func (f fakeReplacements) Validate(_ context.Context, ticket string, _ time.Time) (string, error) {
	ldi, ok := f[ticket]
	if !ok {
		return "", identity.ErrReplacementNotAuthorized
	}
	delete(f, ticket)
	return ldi, nil
}

// newTestService creates a Service wired with in-memory IssuanceLog and
// RevocationQueue, using trustDomain="margo.example.com" and svidTTL=1h.
func newTestService(t *testing.T, repo identity.Repository, issuer identity.Issuer, policy identity.PolicyEngine, replacement identity.ReplacementValidator) (*identity.Service, *memIssuanceLog, *memRevocations) {
	t.Helper()
	log := &memIssuanceLog{}
	rq := newMemRevocations()
	svc := identity.NewService(repo, issuer, policy, replacement, log, rq, "margo.example.com", time.Hour)
	return svc, log, rq
}

// --- in-memory Issuer stub ---

type stubIssuer struct {
	mu           sync.Mutex
	serial       int64
	err          error
	calls        int
	lastSPIFFEID string
}

func (s *stubIssuer) IssueX509SVID(_ context.Context, spiffeID string, _ []byte, ttl time.Duration) ([][]byte, time.Time, error) {
	s.mu.Lock()
	s.calls++
	s.lastSPIFFEID = spiffeID
	s.serial++
	serial := s.serial
	s.mu.Unlock()
	if s.err != nil {
		return nil, time.Time{}, s.err
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	now := time.Now()
	notAfter := now.Add(ttl)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: spiffeID},
		NotBefore:    now,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, time.Time{}, err
	}
	return [][]byte{der}, notAfter, nil
}

// --- helpers ---

func buildValidCSR(t *testing.T) *csr.CSR {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.CertificateRequest{
		Subject:            pkix.Name{CommonName: "x"},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}
	parsed, err := csr.Parse(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatalf("csr.Parse: %v", err)
	}
	return parsed
}

func esiOf(s string) []byte {
	sum := sha256.Sum256([]byte(s))
	return sum[:]
}

// --- tests ---

func TestService_Enroll_InitialReturns201(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}
	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), identity.NoReplacements{})

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("dev-1")}

	out, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if out.StatusCode != 201 {
		t.Errorf("status = %d, want 201", out.StatusCode)
	}
	if out.LDI.UUID == "" {
		t.Error("LDI UUID not set")
	}
	if len(out.CertChainDER) == 0 {
		t.Error("CertChainDER empty")
	}
	if issuer.calls != 1 {
		t.Errorf("issuer calls = %d, want 1", issuer.calls)
	}
}

func TestService_Enroll_SameESIReturns200(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}
	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), identity.NoReplacements{})

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("dev-1")}

	if _, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil); err != nil {
		t.Fatalf("first: %v", err)
	}
	out, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if out.StatusCode != 200 {
		t.Errorf("status = %d, want 200", out.StatusCode)
	}
	if issuer.calls != 2 {
		t.Errorf("issuer calls = %d, want 2", issuer.calls)
	}
}

func TestService_Enroll_KeyRotationBlocked(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}
	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: false}
	svc, _, _ := newTestService(t, repo, issuer, policy, identity.NoReplacements{})

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("dev-rot")}
	if _, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil); err != nil {
		t.Fatalf("first: %v", err)
	}

	_, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
	if !errors.Is(err, identity.ErrKeyRotationNotPermitted) {
		t.Errorf("err = %v, want ErrKeyRotationNotPermitted", err)
	}
}

func TestService_Enroll_NewLDIBlocked(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}
	policy := identity.StaticPolicy{NewLDI: false, KeyRotation: true}
	svc, _, _ := newTestService(t, repo, issuer, policy, identity.NoReplacements{})

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("dev-blocked")}
	_, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
	if !errors.Is(err, identity.ErrNewLDIForbidden) {
		t.Errorf("err = %v, want ErrNewLDIForbidden", err)
	}
}

func TestService_Enroll_ReplacementReturns200(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}

	// Seed an existing LDI under the "old" ESI.
	seededLDI, err := repo.CreateLDIAndBind(context.Background(), esiOf("old"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Make the next CreateLDIAndBind fail if accidentally called - replacement
	// must NOT mint a new LDI.
	repo.nextID = func() string { t.Fatal("replacement must not create a new LDI"); return "" }

	tickets := fakeReplacements{"good-ticket": seededLDI.UUID}
	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), tickets)

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("new")}
	repl := &common.ReplacementAuthorizationDTO{Method: common.ReplacementAuthMethodOperatorTicket, Proof: common.ReplacementAuthorizationProof{Ticket: "good-ticket"}}

	out, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), repl)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if out.StatusCode != 200 {
		t.Errorf("status = %d, want 200 (replacement)", out.StatusCode)
	}
	if out.LDI.UUID != seededLDI.UUID {
		t.Errorf("LDI = %s, want %s (same LDI, new ESI)", out.LDI.UUID, seededLDI.UUID)
	}

	// Old ESI must now be retired; new ESI must resolve as active on the same LDI.
	oldBinding, _, err := repo.LookupByESI(context.Background(), esiOf("old"))
	if err != nil {
		t.Fatalf("old ESI lookup: %v", err)
	}
	if oldBinding.Status != identity.ESIBindingRetired {
		t.Errorf("old ESI status = %q, want retired", oldBinding.Status)
	}
	newBinding, newLDI, err := repo.LookupByESI(context.Background(), esiOf("new"))
	if err != nil {
		t.Fatalf("new ESI lookup: %v", err)
	}
	if newBinding.Status != identity.ESIBindingActive {
		t.Errorf("new ESI status = %q, want active", newBinding.Status)
	}
	if newLDI.UUID != seededLDI.UUID {
		t.Errorf("new ESI bound to %s, want %s", newLDI.UUID, seededLDI.UUID)
	}
}

func TestService_Enroll_UnsupportedReplacementMethod(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}
	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), identity.NoReplacements{})

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("x")}
	repl := &common.ReplacementAuthorizationDTO{Method: "urn:margo:replacement-auth:something-else:v1"}
	_, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), repl)
	if !errors.Is(err, identity.ErrUnsupportedReplacementMethod) {
		t.Errorf("err = %v, want ErrUnsupportedReplacementMethod", err)
	}
}

func TestService_Enroll_ReplacementNotAuthorized(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}
	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), identity.NoReplacements{})

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("x")}
	repl := &common.ReplacementAuthorizationDTO{Method: common.ReplacementAuthMethodOperatorTicket, Proof: common.ReplacementAuthorizationProof{Ticket: "bogus"}}
	_, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), repl)
	if !errors.Is(err, identity.ErrReplacementNotAuthorized) {
		t.Errorf("err = %v, want ErrReplacementNotAuthorized", err)
	}
}

// TestService_Enroll_SpuriousReplacementOnReEnrollmentRejected pins the SUP §5
// rule that replacementAuthorization MUST be absent for ordinary
// re-enrollment, and the refactor guarantee that a legitimate single-use
// ticket is NOT consumed when the rule is violated.
func TestService_Enroll_SpuriousReplacementOnReEnrollmentRejected(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}

	// Seed an existing LDI; the device is re-enrolling with the same ESI.
	seededLDI, err := repo.CreateLDIAndBind(context.Background(), esiOf("same"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	tickets := fakeReplacements{"unused": seededLDI.UUID}
	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), tickets)

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("same")}
	repl := &common.ReplacementAuthorizationDTO{Method: common.ReplacementAuthMethodOperatorTicket, Proof: common.ReplacementAuthorizationProof{Ticket: "unused"}}
	_, err = svc.Enroll(context.Background(), vb, buildValidCSR(t), repl)
	if !errors.Is(err, identity.ErrSpuriousReplacementAuthorization) {
		t.Fatalf("err = %v, want ErrSpuriousReplacementAuthorization", err)
	}

	// The ticket must still be usable afterwards. Validate it directly.
	if _, err := tickets.Validate(context.Background(), "unused", time.Now()); err != nil {
		t.Errorf("ticket was consumed by a rejected re-enrollment: %v", err)
	}
}

// TestService_Enroll_RetiredESIRejected pins SUP §4's "retired ESI MUST NOT be
// sufficient to obtain new SVIDs" rule: a factory cert whose ESI was retired
// by a prior replacement cannot initiate a fresh enrollment, even without a
// replacementAuthorization.
func TestService_Enroll_RetiredESIRejected(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}

	seededLDI, err := repo.CreateLDIAndBind(context.Background(), esiOf("old"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := repo.Rebind(context.Background(), seededLDI.UUID, esiOf("new"), common.BootstrapMethodFactoryCertMTLS); err != nil {
		t.Fatalf("rebind: %v", err)
	}

	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), identity.NoReplacements{})
	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("old")}

	_, err = svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
	if !errors.Is(err, identity.ErrReplacementNotAuthorized) {
		t.Errorf("err = %v, want ErrReplacementNotAuthorized", err)
	}
}

// errOnRecordLog wraps an IssuanceLog and injects an error on Record calls.
type errOnRecordLog struct {
	inner identity.IssuanceLog
	err   error
}

func (l *errOnRecordLog) Record(_ context.Context, _ identity.IssuedSVIDRecord) error {
	return l.err
}
func (l *errOnRecordLog) ActiveBySPIFFEID(ctx context.Context, id string, now time.Time) ([]identity.IssuedSVIDRecord, error) {
	return l.inner.ActiveBySPIFFEID(ctx, id, now)
}
func (l *errOnRecordLog) MarkRevoked(ctx context.Context, serial string, when time.Time) error {
	return l.inner.MarkRevoked(ctx, serial, when)
}
func (l *errOnRecordLog) LeafByActiveSPIFFEID(ctx context.Context, id string, now time.Time) (identity.IssuedSVIDRecord, error) {
	return l.inner.LeafByActiveSPIFFEID(ctx, id, now)
}

// TestService_Enroll_InvalidTicketOnInitialBranch pins that tickets are still
// validated and consumed correctly on the genuine replacement branch (ESI not
// previously bound).
func TestService_Enroll_InvalidTicketOnInitialBranch(t *testing.T) {
	repo := newMemRepo()
	issuer := &stubIssuer{}

	seededLDI, err := repo.CreateLDIAndBind(context.Background(), esiOf("old"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	tickets := fakeReplacements{"good": seededLDI.UUID}
	svc, _, _ := newTestService(t, repo, issuer, identity.DefaultPolicy(), tickets)

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("new")}
	repl := &common.ReplacementAuthorizationDTO{Method: common.ReplacementAuthMethodOperatorTicket, Proof: common.ReplacementAuthorizationProof{Ticket: "good"}}
	if _, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), repl); err != nil {
		t.Fatalf("first replacement: %v", err)
	}
	// Ticket was consumed on the successful replacement; a second attempt must fail.
	vb2 := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("newer")}
	if _, err := svc.Enroll(context.Background(), vb2, buildValidCSR(t), repl); !errors.Is(err, identity.ErrReplacementNotAuthorized) {
		t.Errorf("err = %v, want ErrReplacementNotAuthorized (ticket already consumed)", err)
	}
}

// TestService_Enroll_SameKeyReEnrollmentBypassesPolicy verifies that a
// re-enrollment presenting the same public key as the active SVID succeeds
// even when AllowKeyRotation is false, because no key rotation is occurring.
func TestService_Enroll_SameKeyReEnrollmentBypassesPolicy(t *testing.T) {
	repo := newMemRepo()
	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: false}
	svc, log, _ := newTestService(t, repo, &stubIssuer{}, policy, identity.NoReplacements{})

	ldi, err := repo.CreateLDIAndBind(context.Background(), esiOf("same-key"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("seed LDI: %v", err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	seedActiveLeaf(t, log, ldi.SPIFFEID, key, time.Now().UTC())

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("same-key")}
	out, err := svc.Enroll(context.Background(), vb, mkCSRForKey(t, key), nil)
	if err != nil {
		t.Fatalf("re-enrollment with same key must succeed regardless of policy: %v", err)
	}
	if out.StatusCode != 200 {
		t.Errorf("status = %d, want 200", out.StatusCode)
	}
}

// TestService_Enroll_ReEnrollmentNoActiveLeafBypassesPolicy verifies the
// post-failure recovery path: when CreateLDIAndBind succeeded but recordIssuance
// failed (leaving an active ESI binding with no issued SVID), the device can
// complete enrollment on retry regardless of AllowKeyRotation policy.
func TestService_Enroll_ReEnrollmentNoActiveLeafBypassesPolicy(t *testing.T) {
	repo := newMemRepo()
	policy := identity.StaticPolicy{NewLDI: true, KeyRotation: false}
	svc, _, _ := newTestService(t, repo, &stubIssuer{}, policy, identity.NoReplacements{})

	// Simulate CreateLDIAndBind having succeeded without any issuance record.
	_, err := repo.CreateLDIAndBind(context.Background(), esiOf("recovering"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
	if err != nil {
		t.Fatalf("seed ESI binding: %v", err)
	}

	vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("recovering")}
	out, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
	if err != nil {
		t.Fatalf("re-enrollment with no active leaf must succeed: %v", err)
	}
	if out.StatusCode != 200 {
		t.Errorf("status = %d, want 200", out.StatusCode)
	}
}

// TestService_Enroll_RecordIssuanceFailure verifies that a failing audit write
// propagates as an error rather than being silently discarded, for all three
// enrollment paths.
func TestService_Enroll_RecordIssuanceFailure(t *testing.T) {
	recordErr := errors.New("audit write failed")

	t.Run("handleInitial", func(t *testing.T) {
		repo := newMemRepo()
		failLog := &errOnRecordLog{inner: &memIssuanceLog{}, err: recordErr}
		svc := identity.NewService(repo, &stubIssuer{}, identity.DefaultPolicy(), identity.NoReplacements{}, failLog, newMemRevocations(), "margo.example.com", time.Hour)

		vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("rec-initial")}
		_, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
		if !errors.Is(err, recordErr) {
			t.Errorf("err = %v, want %v", err, recordErr)
		}
	})

	t.Run("handleReEnrollment", func(t *testing.T) {
		repo := newMemRepo()
		// Seed ESI binding with no issuance record - simulates a prior partial failure.
		if _, err := repo.CreateLDIAndBind(context.Background(), esiOf("rec-reenroll"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com"); err != nil {
			t.Fatalf("seed: %v", err)
		}
		failLog := &errOnRecordLog{inner: &memIssuanceLog{}, err: recordErr}
		svc := identity.NewService(repo, &stubIssuer{}, identity.DefaultPolicy(), identity.NoReplacements{}, failLog, newMemRevocations(), "margo.example.com", time.Hour)

		vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("rec-reenroll")}
		_, err := svc.Enroll(context.Background(), vb, buildValidCSR(t), nil)
		if !errors.Is(err, recordErr) {
			t.Errorf("err = %v, want %v", err, recordErr)
		}
	})

	t.Run("handleReplacement", func(t *testing.T) {
		repo := newMemRepo()
		seededLDI, err := repo.CreateLDIAndBind(context.Background(), esiOf("old-rec"), common.BootstrapMethodFactoryCertMTLS, "margo.example.com")
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		failLog := &errOnRecordLog{inner: &memIssuanceLog{}, err: recordErr}
		tickets := fakeReplacements{"ticket-rec": seededLDI.UUID}
		svc := identity.NewService(repo, &stubIssuer{}, identity.DefaultPolicy(), tickets, failLog, newMemRevocations(), "margo.example.com", time.Hour)

		vb := bootstrap.VerifiedBootstrap{Method: common.BootstrapMethodFactoryCertMTLS, ESI: esiOf("new-rec")}
		repl := &common.ReplacementAuthorizationDTO{Method: common.ReplacementAuthMethodOperatorTicket, Proof: common.ReplacementAuthorizationProof{Ticket: "ticket-rec"}}
		_, err = svc.Enroll(context.Background(), vb, buildValidCSR(t), repl)
		if !errors.Is(err, recordErr) {
			t.Errorf("err = %v, want %v", err, recordErr)
		}
	})
}
