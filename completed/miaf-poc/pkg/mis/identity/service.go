package identity

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/csr"
)

// Typed errors. Handlers map each to a specific SUP Appendix B row.
var (
	ErrNewLDIForbidden              = errors.New("identity: policy forbids minting a new LDI")
	ErrKeyRotationNotPermitted      = errors.New("identity: policy forbids key rotation on re-enrollment")
	ErrUnsupportedReplacementMethod = errors.New("identity: unsupported replacementAuthorization.method")
	// ErrSpuriousReplacementAuthorization is returned when a client presents a
	// replacementAuthorization alongside an ESI that is already actively bound
	// (i.e. an ordinary re-enrollment). SUP §5 requires replacementAuthorization
	// to be absent for initial enrollment and ordinary re-enrollment. Handlers
	// map this to 400 Bad Request so the client can recover by retrying without
	// the replacement block; importantly, the presented ticket is NOT consumed.
	ErrSpuriousReplacementAuthorization = errors.New("identity: replacementAuthorization must be absent for ordinary re-enrollment")
)

// Issuer mints an X.509-SVID for the given SPIFFE ID. The device's CSR
// conveys the public key only; the issuer sets the URI SAN authoritatively
// per SUP §5. Returns the chain as DER blocks (leaf first, then any
// intermediates) and the leaf's NotAfter. *ca.IntermediateIssuer satisfies
// this - it signs in-process against an operator-provisioned intermediate.
type Issuer interface {
	IssueX509SVID(ctx context.Context, spiffeID string, csrDER []byte, ttl time.Duration) ([][]byte, time.Time, error)
}

// Service owns the SUP §5 enrollment-and-issuance sequence.
type Service struct {
	repo        Repository
	issuer      Issuer
	policy      PolicyEngine
	replacement ReplacementValidator
	issuance    IssuanceLog
	revocations RevocationQueue
	trustDomain string
	svidTTL     time.Duration
	now         func() time.Time
}

// NewService constructs a Service.
func NewService(
	repo Repository,
	issuer Issuer,
	policy PolicyEngine,
	replacement ReplacementValidator,
	issuance IssuanceLog,
	revocations RevocationQueue,
	trustDomain string,
	svidTTL time.Duration,
) *Service {
	if repo == nil || issuer == nil || policy == nil || replacement == nil || issuance == nil || revocations == nil {
		panic("identity.NewService: all dependencies required")
	}
	return &Service{
		repo:        repo,
		issuer:      issuer,
		policy:      policy,
		replacement: replacement,
		issuance:    issuance,
		revocations: revocations,
		trustDomain: trustDomain,
		svidTTL:     svidTTL,
		now:         time.Now,
	}
}

// SetServiceNow overrides the clock source. Test-only.
func SetServiceNow(s *Service, f func() time.Time) { s.now = f }

func (s *Service) recordIssuance(ctx context.Context, ldi LDI, chainDER [][]byte, method string, notAfter time.Time) error {
	if len(chainDER) == 0 {
		return errors.New("identity: empty chain cannot be audited")
	}
	leaf, err := x509.ParseCertificate(chainDER[0])
	if err != nil {
		return fmt.Errorf("identity: parse leaf for audit: %w", err)
	}
	fp := sha256.Sum256(chainDER[0])
	rec := IssuedSVIDRecord{
		SerialHex:      strings.ToUpper(leaf.SerialNumber.Text(16)),
		FingerprintHex: hex.EncodeToString(fp[:]),
		SPIFFEID:       ldi.SPIFFEID,
		LDIUUID:        ldi.UUID,
		Method:         method,
		IssuedAt:       s.now().UTC(),
		NotBefore:      leaf.NotBefore.UTC(),
		NotAfter:       notAfter.UTC(),
		LeafDER:        chainDER[0],
	}
	return s.issuance.Record(ctx, rec)
}

func (s *Service) revokeActivePriors(ctx context.Context, spiffeID, reason string) ([]RevocationEntry, error) {
	priors, err := s.issuance.ActiveBySPIFFEID(ctx, spiffeID, s.now())
	if err != nil {
		return nil, err
	}
	out := make([]RevocationEntry, 0, len(priors))
	for _, p := range priors {
		newly, err := s.revocations.Revoke(ctx, p.SerialHex, p.FingerprintHex, reason, p.NotAfter, s.now())
		if err != nil {
			return out, err
		}
		_ = s.issuance.MarkRevoked(ctx, p.SerialHex, s.now())
		if newly {
			out = append(out, RevocationEntry{
				SerialHex:      p.SerialHex,
				FingerprintHex: p.FingerprintHex,
				RevokedAt:      s.now(),
				Reason:         reason,
				NotAfter:       p.NotAfter,
			})
		}
	}
	return out, nil
}

// EnrollmentResult is what the handler needs to render a response.
type EnrollmentResult struct {
	StatusCode   int
	LDI          LDI
	CertChainDER [][]byte
	ExpiresAt    time.Time
}

// Enroll executes SUP §5 "MIS Validation and Processing Logic".
//
// (Replacement) ticket validation is deferred until we've decided the presented ESI is
// genuinely unbound so a legitimate single-use operator ticket is never
// consumed on an ordinary re-enrollment (which per SUP §5 MUST NOT carry a
// replacementAuthorization at all). Retired ESIs - ESIs that once belonged to
// an LDI but were displaced by a successful replacement - are rejected
// because SUP §4 says a retired ESI "MUST NOT be sufficient to obtain new
// SVIDs for the LDI unless explicitly re-authorized"; there is no
// re-authorization path defined for the retired-ESI presenter in SUP v1.
func (s *Service) Enroll(ctx context.Context, vb bootstrap.VerifiedBootstrap, parsedCSR *csr.CSR, repl *common.ReplacementAuthorizationDTO) (EnrollmentResult, error) {
	existingBinding, existingLDI, err := s.repo.LookupByESI(ctx, vb.ESI)
	switch {
	case errors.Is(err, ErrNotFound):
		// ESI has never been bound: initial enrollment, or replacement onto an
		// existing LDI selected by an operator ticket.
		if repl != nil {
			return s.handleReplacement(ctx, vb, parsedCSR, repl)
		}
		return s.handleInitial(ctx, vb, parsedCSR)
	case err != nil:
		return EnrollmentResult{}, err
	default:
		switch existingBinding.Status {
		case ESIBindingActive:
			if repl != nil {
				return EnrollmentResult{}, ErrSpuriousReplacementAuthorization
			}
			return s.handleReEnrollment(ctx, existingLDI, parsedCSR, vb.Method)
		case ESIBindingRetired:
			return EnrollmentResult{}, ErrReplacementNotAuthorized
		default:
			return EnrollmentResult{}, fmt.Errorf("identity: unexpected ESI binding status %q", existingBinding.Status)
		}
	}
}

func (s *Service) handleInitial(ctx context.Context, vb bootstrap.VerifiedBootstrap, parsed *csr.CSR) (EnrollmentResult, error) {
	if !s.policy.AllowNewLDI() {
		return EnrollmentResult{}, ErrNewLDIForbidden
	}
	ldi, err := s.repo.CreateLDIAndBind(ctx, vb.ESI, vb.Method, s.trustDomain)
	if err != nil {
		return EnrollmentResult{}, err
	}
	chain, expires, err := s.issuer.IssueX509SVID(ctx, ldi.SPIFFEID, parsed.DER(), s.svidTTL)
	if err != nil {
		return EnrollmentResult{}, err
	}
	if err := s.recordIssuance(ctx, ldi, chain, vb.Method, expires); err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{StatusCode: 201, LDI: ldi, CertChainDER: chain, ExpiresAt: expires}, nil
}

func (s *Service) handleReplacement(ctx context.Context, vb bootstrap.VerifiedBootstrap, parsed *csr.CSR, repl *common.ReplacementAuthorizationDTO) (EnrollmentResult, error) {
	if repl.Method != common.ReplacementAuthMethodOperatorTicket {
		return EnrollmentResult{}, ErrUnsupportedReplacementMethod
	}
	// Validation consumes the ticket; this is deliberately only reachable
	// after the ESI has been confirmed unbound (see Enroll).
	ldiUUID, err := s.replacement.Validate(ctx, repl.Proof.Ticket, s.now())
	if err != nil {
		return EnrollmentResult{}, err
	}
	target, err := s.repo.GetLDI(ctx, ldiUUID)
	if errors.Is(err, ErrNotFound) {
		return EnrollmentResult{}, ErrReplacementNotAuthorized
	}
	if err != nil {
		return EnrollmentResult{}, err
	}
	if target.Status != LDIStatusActive {
		return EnrollmentResult{}, ErrReplacementNotAuthorized
	}
	if _, err := s.repo.Rebind(ctx, target.UUID, vb.ESI, vb.Method); err != nil {
		return EnrollmentResult{}, err
	}
	_, _ = s.revokeActivePriors(ctx, target.SPIFFEID, "replacement")
	chain, expires, err := s.issuer.IssueX509SVID(ctx, target.SPIFFEID, parsed.DER(), s.svidTTL)
	if err != nil {
		return EnrollmentResult{}, err
	}
	if err := s.recordIssuance(ctx, target, chain, vb.Method, expires); err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{StatusCode: 200, LDI: target, CertChainDER: chain, ExpiresAt: expires}, nil
}

func (s *Service) handleReEnrollment(ctx context.Context, ldi LDI, parsed *csr.CSR, method string) (EnrollmentResult, error) {
	if ldi.Status != LDIStatusActive {
		return EnrollmentResult{}, ErrReplacementNotAuthorized
	}
	current, err := s.issuance.LeafByActiveSPIFFEID(ctx, ldi.SPIFFEID, s.now())
	switch {
	case errors.Is(err, ErrNotFound):
		// No active SVID — not a key rotation (e.g. retry after CA failure).
	case err != nil:
		return EnrollmentResult{}, err
	default:
		keyChanged, err := renewalKeyChanged(current.LeafDER, parsed.Inner())
		if err != nil {
			return EnrollmentResult{}, err
		}
		if keyChanged && !s.policy.AllowKeyRotation() {
			return EnrollmentResult{}, ErrKeyRotationNotPermitted
		}
	}
	_, _ = s.revokeActivePriors(ctx, ldi.SPIFFEID, "re-enrollment")
	chain, expires, err := s.issuer.IssueX509SVID(ctx, ldi.SPIFFEID, parsed.DER(), s.svidTTL)
	if err != nil {
		return EnrollmentResult{}, err
	}
	if err := s.recordIssuance(ctx, ldi, chain, method, expires); err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{StatusCode: 200, LDI: ldi, CertChainDER: chain, ExpiresAt: expires}, nil
}

// Renew issues a new X.509 SVID for an existing LDI identified by SPIFFE ID.
//
// Invariants:
//   - spiffeID comes from the URL path (SUP §"SVID Renewal Endpoint"); caller
//     has already verified it matches the authenticated credential.
//   - key rotation is derived from the active issued leaf associated with the
//     SPIFFE ID, not from any particular transport credential. This keeps mTLS
//     and bearer-auth renewal on the same policy path.
//   - Rate-limit enforcement is the caller's responsibility; Renew always
//     issues when permitted by policy.
func (s *Service) Renew(ctx context.Context, spiffeID string, parsed *csr.CSR) (EnrollmentResult, error) {
	ldi, err := s.repo.GetLDIBySPIFFEID(ctx, spiffeID)
	if err != nil {
		return EnrollmentResult{}, err
	}
	if ldi.Status != LDIStatusActive {
		return EnrollmentResult{}, ErrNotFound
	}
	current, err := s.issuance.LeafByActiveSPIFFEID(ctx, spiffeID, s.now())
	if err != nil {
		return EnrollmentResult{}, err
	}
	keyChanged, err := renewalKeyChanged(current.LeafDER, parsed.Inner())
	if err != nil {
		return EnrollmentResult{}, err
	}
	if keyChanged && !s.policy.AllowKeyRotation() {
		return EnrollmentResult{}, ErrKeyRotationNotPermitted
	}
	if keyChanged {
		_, _ = s.revokeActivePriors(ctx, ldi.SPIFFEID, "key-rotation")
	}
	chain, expires, err := s.issuer.IssueX509SVID(ctx, ldi.SPIFFEID, parsed.DER(), s.svidTTL)
	if err != nil {
		return EnrollmentResult{}, err
	}
	if err := s.recordIssuance(ctx, ldi, chain, "renewal", expires); err != nil {
		return EnrollmentResult{}, err
	}
	return EnrollmentResult{StatusCode: 200, LDI: ldi, CertChainDER: chain, ExpiresAt: expires}, nil
}

func renewalKeyChanged(activeLeafDER []byte, req *x509.CertificateRequest) (bool, error) {
	leaf, err := x509.ParseCertificate(activeLeafDER)
	if err != nil {
		return false, fmt.Errorf("parse active leaf: %w", err)
	}
	leafDER, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return false, fmt.Errorf("marshal active leaf public key: %w", err)
	}
	csrDER, err := x509.MarshalPKIXPublicKey(req.PublicKey)
	if err != nil {
		return false, fmt.Errorf("marshal renewal csr public key: %w", err)
	}
	h1 := sha256.Sum256(leafDER)
	h2 := sha256.Sum256(csrDER)
	return h1 != h2, nil
}

// RevokeSPIFFEID revokes all active SVIDs for the given SPIFFE ID and sets the
// LDI status to Revoked. Returns newly revoked entries and already-revoked
// serial hex strings.
func (s *Service) RevokeSPIFFEID(ctx context.Context, spiffeID, reason string) (newlyRevoked []RevocationEntry, alreadyRevoked []string, err error) {
	ldi, err := s.repo.GetLDIBySPIFFEID(ctx, spiffeID)
	if err != nil {
		return nil, nil, err
	}
	priors, err := s.issuance.ActiveBySPIFFEID(ctx, spiffeID, s.now())
	if err != nil {
		return nil, nil, err
	}
	for _, p := range priors {
		newly, rerr := s.revocations.Revoke(ctx, p.SerialHex, p.FingerprintHex, reason, p.NotAfter, s.now())
		if rerr != nil {
			return newlyRevoked, alreadyRevoked, rerr
		}
		_ = s.issuance.MarkRevoked(ctx, p.SerialHex, s.now())
		if newly {
			newlyRevoked = append(newlyRevoked, RevocationEntry{
				SerialHex:      p.SerialHex,
				FingerprintHex: p.FingerprintHex,
				RevokedAt:      s.now(),
				Reason:         reason,
				NotAfter:       p.NotAfter,
			})
		} else {
			alreadyRevoked = append(alreadyRevoked, p.SerialHex)
		}
	}
	if err := s.repo.SetLDIStatus(ctx, ldi.UUID, LDIStatusRevoked); err != nil {
		return newlyRevoked, alreadyRevoked, err
	}
	return newlyRevoked, alreadyRevoked, nil
}
