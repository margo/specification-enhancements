package http

import (
	"bytes"
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
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt/replay"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/jwtexchange"
)

// JWTSVIDConfig bundles dependencies for the JWT SVID Exchange handler.
type JWTSVIDConfig struct {
	Service      *identity.Service
	Signer       identity.JWTSigner
	IssuanceLog  identity.JWTSVIDIssuanceLog
	LeafLookup   jwtexchange.LeafLookup
	ReplayStore  replay.Store
	RateLimiter  identity.RenewalRateLimiter
	IssuerURL    string
	EndpointBase string
	MaxTTL       time.Duration
	DefaultTTL   time.Duration
	ClockSkew    time.Duration
	Logger       *slog.Logger
	Now          func() time.Time
}

// NewJWTSVIDHandler builds the POST /api/v1/identities/{spiffeIdEncoded}/jwt-svid handler.
func NewJWTSVIDHandler(cfg JWTSVIDConfig) http.Handler {
	for _, req := range []struct {
		name string
		ok   bool
	}{
		{"Service", cfg.Service != nil},
		{"Signer", cfg.Signer != nil},
		{"IssuanceLog", cfg.IssuanceLog != nil},
		{"LeafLookup", cfg.LeafLookup != nil},
		{"RateLimiter", cfg.RateLimiter != nil},
		{"IssuerURL", cfg.IssuerURL != ""},
		{"EndpointBase", cfg.EndpointBase != ""},
		{"MaxTTL", cfg.MaxTTL > 0},
	} {
		if !req.ok {
			panic("http: JWTSVIDConfig." + req.name + " is required")
		}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = cfg.MaxTTL
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleJWTSVID(w, r, cfg)
	})
}

func handleJWTSVID(w http.ResponseWriter, r *http.Request, cfg JWTSVIDConfig) {
	encoded := r.PathValue("spiffeIdEncoded")
	spiffeID, err := DecodeSPIFFEIDEncoded(encoded)
	if err != nil {
		WriteProblem(w, r, errpkg.InvalidSVIDRequest.Problem("invalid {spiffeIdEncoded}: "+err.Error()))
		return
	}

	// Reject Authorization: Bearer ... unconditionally per SUP §"JWT SVID Exchange Endpoint".
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
			WithDetail("MIS MUST NOT accept JWT SVID Bearer authentication on /jwt-svid"))
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		WriteProblem(w, r, errpkg.New(http.StatusBadRequest, "Bad Request").
			WithDetail("request body exceeds 64 KiB or is unreadable"))
		return
	}
	var req common.JWTSVIDExchangeRequestDTO
	if err := json.Unmarshal(body, &req); err != nil {
		WriteProblem(w, r, errpkg.InvalidSVIDRequest.Problem("invalid JSON: "+err.Error()))
		return
	}

	if err := jwtexchange.ValidateAudience(req.Audience); err != nil {
		WriteProblem(w, r, errpkg.InvalidAudience.Problem(err.Error()))
		return
	}
	if req.TTLSeconds < 0 {
		WriteProblem(w, r, errpkg.InvalidSVIDRequest.Problem("ttl must be >= 0"))
		return
	}

	// Authenticate.
	authenticated := false
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		peer := r.TLS.PeerCertificates[0]
		if len(peer.URIs) == 0 || peer.URIs[0].String() != spiffeID {
			WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
				WithDetail("peer certificate URI SAN does not match {spiffeIdEncoded}"))
			return
		}
		authenticated = true
	}
	if !authenticated {
		if strings.TrimSpace(req.ClientAssertion) == "" {
			WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
				WithDetail("authentication required: present mTLS client SVID or client_assertion JWT"))
			return
		}
		expectedAud := strings.TrimRight(cfg.EndpointBase, "/") + "/api/v1/identities/" + encoded + "/jwt-svid"
		if _, err := jwtexchange.VerifyClientAssertion(r.Context(), jwtexchange.VerifyConfig{
			LeafLookup:  cfg.LeafLookup,
			Replay:      cfg.ReplayStore,
			Now:         cfg.Now,
			ExpectedAud: expectedAud,
			MaxLifetime: 5 * time.Minute,
			ClockSkew:   cfg.ClockSkew,
		}, spiffeID, req.ClientAssertion); err != nil {
			WriteProblem(w, r, errpkg.New(http.StatusUnauthorized, "Unauthorized").
				WithDetail("client_assertion validation failed: "+err.Error()))
			return
		}
	}

	// Rate-limit (shared with renewal).
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
			fmt.Sprintf("per-identity rate limit exceeded; retry after %s", retryAfter.Round(time.Second))))
		return
	}

	// Issue.
	ttl := time.Duration(req.TTLSeconds) * time.Second
	res, err := cfg.Service.IssueJWTSVID(r.Context(), cfg.Signer, cfg.IssuanceLog,
		identity.JWTSVIDConfig{Issuer: cfg.IssuerURL, DefaultTTL: cfg.DefaultTTL, MaxTTL: cfg.MaxTTL},
		spiffeID, req.Audience, ttl)
	if err != nil {
		switch {
		case errors.Is(err, identity.ErrNotFound):
			WriteProblem(w, r, errpkg.New(http.StatusNotFound, "Not Found").
				WithDetail("no active identity is bound to "+spiffeID))
		default:
			WriteProblem(w, r, errpkg.New(http.StatusInternalServerError, "Internal Server Error").
				WithDetail("jwt-svid issuance failed: "+err.Error()))
		}
		return
	}

	if rerr := cfg.RateLimiter.Record(r.Context(), spiffeID, now); rerr != nil {
		cfg.Logger.Warn("jwt-svid rate-limit record failed",
			"request_id", RequestIDFromContext(r.Context()),
			"spiffe_id", spiffeID,
			"err", rerr.Error())
	}

	resp := common.JWTSVIDExchangeResponseDTO{
		JWT:       res.JWT,
		ExpiresAt: res.ExpiresAt.UTC().Format(time.RFC3339),
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&resp); err != nil {
		WriteProblem(w, r, errpkg.New(http.StatusInternalServerError, "Internal Server Error").
			WithDetail("encode response: "+err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(buf.Bytes())

	cfg.Logger.Info("jwt-svid issued",
		"request_id", RequestIDFromContext(r.Context()),
		"spiffe_id", spiffeID,
		"audiences", req.Audience,
		"ttl_seconds", int(res.ExpiresAt.Sub(now).Seconds()))
}
