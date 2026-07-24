package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/csr"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/spiffe/go-spiffe/v2/bundle/jwtbundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
)

// RenewalBearerVerifier validates a JWT-SVID bearer token and returns the
// authenticated SPIFFE ID.
type RenewalBearerVerifier interface {
	Verify(ctx context.Context, token, audience string) (string, error)
}

type localRenewalBearerVerifier struct {
	source BundleSource
}

// NewLocalRenewalBearerVerifier validates bearer JWT-SVIDs against the local
// trust anchors already published via the Bundle Map endpoint.
func NewLocalRenewalBearerVerifier(source BundleSource) RenewalBearerVerifier {
	if source == nil {
		panic("http: BundleSource is required")
	}
	return &localRenewalBearerVerifier{source: source}
}

// RenewalConfig bundles dependencies required to construct the renewal handler.
type RenewalConfig struct {
	Service         *identity.Service
	RateLimiter     identity.RenewalRateLimiter
	BearerVerifier  RenewalBearerVerifier
	EndpointBaseURL string
	Logger          *slog.Logger
	// Now overrides the clock source for tests. Zero value uses time.Now.
	Now func() time.Time
}

// NewRenewalHandler builds the POST /api/v1/identities/{spiffeIdEncoded}/renewal handler.
func NewRenewalHandler(cfg RenewalConfig) http.Handler {
	if cfg.Service == nil {
		panic("http: RenewalConfig.Service is required")
	}
	if cfg.RateLimiter == nil {
		panic("http: RenewalConfig.RateLimiter is required")
	}
	if cfg.BearerVerifier != nil && cfg.EndpointBaseURL == "" {
		panic("http: RenewalConfig.EndpointBaseURL is required when BearerVerifier is configured")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleRenewal(w, r, cfg)
	})
}

func handleRenewal(w http.ResponseWriter, r *http.Request, cfg RenewalConfig) {
	// Decode {spiffeIdEncoded} path param.
	encoded := r.PathValue("spiffeIdEncoded")
	spiffeID, err := DecodeSPIFFEIDEncoded(encoded)
	if err != nil {
		WriteProblem(w, r, errpkg.InvalidSVIDRequest.Problem("invalid {spiffeIdEncoded}: "+err.Error()))
		return
	}

	// Authenticate with the current X.509 SVID when present, otherwise fall
	// back to JWT-SVID bearer authentication.
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		peer := r.TLS.PeerCertificates[0]
		if len(peer.URIs) == 0 || peer.URIs[0].String() != spiffeID {
			WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
				WithDetail("peer certificate URI SAN does not match {spiffeIdEncoded}"))
			return
		}
	} else if auth := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(auth, "Bearer ") {
		if cfg.BearerVerifier == nil {
			WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
				WithDetail("JWT SVID bearer authentication is not configured on this MIS instance"))
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		audience := strings.TrimRight(cfg.EndpointBaseURL, "/") + "/api/v1/identities/" + encoded + "/renewal"
		authenticatedSPIFFEID, err := cfg.BearerVerifier.Verify(r.Context(), token, audience)
		if err != nil {
			WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
				WithDetail("JWT SVID bearer validation failed: "+err.Error()))
			return
		}
		if authenticatedSPIFFEID != spiffeID {
			WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
				WithDetail("JWT SVID subject does not match {spiffeIdEncoded}"))
			return
		}
	} else {
		WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
			WithDetail("authentication required: present the current SVID via mTLS or a JWT SVID bearer token"))
		return
	}

	// Read and unmarshal body.
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		WriteProblem(w, r, errpkg.New(http.StatusBadRequest, "Bad Request").
			WithDetail("request body exceeds 64 KiB or is unreadable"))
		return
	}
	var req common.EnrollmentRequestDTO
	if err := json.Unmarshal(body, &req); err != nil {
		WriteProblem(w, r, errpkg.New(http.StatusBadRequest, "Bad Request").
			WithDetail("invalid JSON: "+err.Error()))
		return
	}

	// SVID profile check.
	if req.SVIDProfileURI != common.SVIDProfileURIX509V1 {
		WriteProblem(w, r, errpkg.UnsupportedSVIDProfile.Problem(
			"renewal requires "+common.SVIDProfileURIX509V1))
		return
	}

	// Parse CSR.
	parsed, err := csr.Parse(req.SVIDRequest.CSR)
	if err != nil {
		writeCSRError(w, r, err)
		return
	}

	// Rate-limit check.
	now := cfg.Now().UTC()
	allowed, retryAfter, err := cfg.RateLimiter.Allow(r.Context(), spiffeID, now)
	if err != nil {
		WriteProblem(w, r, errpkg.New(http.StatusInternalServerError, "Internal Server Error").
			WithDetail("rate-limit lookup failed: "+err.Error()))
		return
	}
	if !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		WriteProblem(w, r, errpkg.TooManyRequests.Problem(
			fmt.Sprintf("per-identity renewal rate limit exceeded; retry after %s", retryAfter.Round(time.Second))))
		return
	}

	// Call renew service.
	result, err := cfg.Service.Renew(r.Context(), spiffeID, parsed)
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrKeyRotationNotPermitted):
			WriteProblem(w, r, errpkg.KeyRotationNotPermitted.Problem(err.Error()))
		case errors.Is(err, identity.ErrNotFound):
			WriteProblem(w, r, errpkg.New(http.StatusNotFound, "Not Found").
				WithDetail("no identity is bound to "+spiffeID))
		default:
			WriteProblem(w, r, errpkg.New(http.StatusInternalServerError, "Internal Server Error").
				WithDetail("renewal failed: "+err.Error()))
		}
		return
	}

	// 8. Record success.
	if rerr := cfg.RateLimiter.Record(r.Context(), spiffeID, now); rerr != nil {
		cfg.Logger.Warn("renewal rate-limit record failed",
			"request_id", RequestIDFromContext(r.Context()),
			"spiffe_id", spiffeID,
			"err", rerr.Error(),
		)
	}

	// Write response.
	resp := common.EnrollmentResponseDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVID: common.X509SVIDDTO{
			CertificateChainPEM: derChainToPEM(result.CertChainDER),
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&resp); err != nil {
		WriteProblem(w, r, errpkg.New(http.StatusInternalServerError, "Internal Server Error").
			WithDetail("encode response: "+err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(result.StatusCode) // 200
	_, _ = w.Write(buf.Bytes())

	// Log renewal.
	cfg.Logger.Info("renewal",
		"request_id", RequestIDFromContext(r.Context()),
		"spiffe_id", spiffeID,
		"status", result.StatusCode,
	)
}

func (v *localRenewalBearerVerifier) Verify(ctx context.Context, token, audience string) (string, error) {
	set, _, err := v.source.GetTrustAnchors(ctx)
	if err != nil {
		return "", err
	}
	trustDomain, err := spiffeid.TrustDomainFromString(set.TrustDomain)
	if err != nil {
		return "", fmt.Errorf("parse trust domain: %w", err)
	}
	bundle := jwtbundle.New(trustDomain)
	for _, authority := range set.JWTAuthorities {
		if err := bundle.AddJWTAuthority(authority.KeyID, authority.PublicKey); err != nil {
			return "", fmt.Errorf("add jwt authority %q: %w", authority.KeyID, err)
		}
	}
	if len(set.JWTAuthorities) == 0 {
		return "", errors.New("no JWT authorities available")
	}
	svid, err := jwtsvid.ParseAndValidate(token, bundle, []string{audience})
	if err != nil {
		return "", err
	}
	return svid.ID.String(), nil
}
