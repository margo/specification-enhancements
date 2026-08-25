package http

import (
	"bytes"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/csr"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// EnrollmentConfig configures the POST /api/v1/identities handler.
type EnrollmentConfig struct {
	Registry *bootstrap.Registry
	Service  *identity.Service
	Logger   *slog.Logger
}

// NewEnrollmentHandler builds the enrollment handler.
func NewEnrollmentHandler(cfg EnrollmentConfig) http.Handler {
	if cfg.Registry == nil {
		panic("http: EnrollmentConfig.Registry is required")
	}
	if cfg.Service == nil {
		panic("http: EnrollmentConfig.Service is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleEnrollment(w, r, cfg)
	})
}

func handleEnrollment(w http.ResponseWriter, r *http.Request, cfg EnrollmentConfig) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10)) // 64 KiB cap
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

	// Per SUP §4 the only permitted SVID profile URI for devices is X.509 v1;
	// a JWT profile URI here MUST yield 422 unsupported-svid-profile.
	if req.SVIDProfileURI != common.SVIDProfileURIX509V1 {
		WriteProblem(w, r, errpkg.UnsupportedSVIDProfile.Problem(
			"device enrollment requires "+common.SVIDProfileURIX509V1))
		return
	}

	validator, err := cfg.Registry.Lookup(req.BootstrapCredential.Method)
	if err != nil {
		WriteProblem(w, r, errpkg.UnsupportedBootstrapMethod.Problem(
			"bootstrap method not supported: "+req.BootstrapCredential.Method))
		return
	}

	outcome, err := validator.Validate(r.Context(), r, req.BootstrapCredential, req.SVIDRequest.CSR, req.SVIDProfileURI)
	if err != nil {
		writeBootstrapError(w, r, err)
		return
	}
	if outcome.Replay != nil {
		writeEnrollmentResponse(w, outcome.Replay.Response, outcome.Replay.StatusCode)
		cfg.Logger.Info("enrollment (replay)",
			"request_id", RequestIDFromContext(r.Context()),
			"method", req.BootstrapCredential.Method,
			"status", outcome.Replay.StatusCode,
		)
		return
	}

	parsed, err := csr.Parse(req.SVIDRequest.CSR)
	if err != nil {
		writeCSRError(w, r, err)
		return
	}

	result, err := cfg.Service.Enroll(r.Context(), outcome.Pending.VB, parsed, req.ReplacementAuthorization)
	if err != nil {
		writeIdentityError(w, r, err)
		return
	}

	resp := common.EnrollmentResponseDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVID: common.X509SVIDDTO{
			CertificateChainPEM: derChainToPEM(result.CertChainDER),
		},
	}
	writeEnrollmentResponse(w, &resp, result.StatusCode)

	if outcome.Pending.Commit != nil {
		if cerr := outcome.Pending.Commit(r.Context(), &resp, result.StatusCode, result.LDI.UUID); cerr != nil {
			cfg.Logger.Warn("enrollment-token commit failed",
				"request_id", RequestIDFromContext(r.Context()),
				"spiffe_id", result.LDI.SPIFFEID,
				"err", cerr.Error(),
			)
		}
	}

	cfg.Logger.Info("enrollment",
		"request_id", RequestIDFromContext(r.Context()),
		"method", outcome.Pending.VB.Method,
		"spiffe_id", result.LDI.SPIFFEID,
		"status", result.StatusCode,
	)
}

func writeEnrollmentResponse(w http.ResponseWriter, resp *common.EnrollmentResponseDTO, status int) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		WriteProblem(w, nil, errpkg.New(http.StatusInternalServerError, "Internal Server Error").
			WithDetail("encode response: "+err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func writeBootstrapError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, factorycertmtls.ErrMissingMTLS),
		errors.Is(err, factorycertmtls.ErrUntrustedChain),
		errors.Is(err, factorycertmtls.ErrUnexpectedProof):
		WriteProblem(w, r, errpkg.InvalidBootstrapProof.Problem(err.Error()))

	case errors.Is(err, factorycertjwt.ErrMissingAssertion),
		errors.Is(err, factorycertjwt.ErrMalformedAssertion),
		errors.Is(err, factorycertjwt.ErrUnsupportedAlg),
		errors.Is(err, factorycertjwt.ErrBadSignature),
		errors.Is(err, factorycertjwt.ErrUntrustedChain),
		errors.Is(err, factorycertjwt.ErrClaimMissing),
		errors.Is(err, factorycertjwt.ErrClaimInvalid),
		errors.Is(err, factorycertjwt.ErrReplay):
		WriteProblem(w, r, errpkg.InvalidBootstrapProof.Problem(err.Error()))

	case errors.Is(err, enrollmenttoken.ErrMissingToken),
		errors.Is(err, enrollmenttoken.ErrInvalidToken):
		WriteProblem(w, r, errpkg.InvalidEnrollmentToken.Problem(err.Error()))

	default:
		WriteProblem(w, r, errpkg.InvalidBootstrapProof.Problem("bootstrap proof rejected"))
	}
}

func writeCSRError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, csr.ErrPEMArmor),
		errors.Is(err, csr.ErrMalformed),
		errors.Is(err, csr.ErrBadSignature):
		WriteProblem(w, r, errpkg.InvalidSVIDRequest.Problem(err.Error()))
	case errors.Is(err, csr.ErrUnsupportedKey),
		errors.Is(err, csr.ErrUnsupportedAlg):
		WriteProblem(w, r, errpkg.UnsupportedSVIDProfile.Problem(err.Error()))
	default:
		WriteProblem(w, r, errpkg.InvalidSVIDRequest.Problem("CSR validation failed"))
	}
}

func writeIdentityError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, identity.ErrUnsupportedReplacementMethod):
		WriteProblem(w, r, errpkg.UnsupportedReplacementAuthorizationMethod.Problem(err.Error()))
	case errors.Is(err, identity.ErrSpuriousReplacementAuthorization):
		WriteProblem(w, r, errpkg.SpuriousReplacementAuthorization.Problem(
			"replacementAuthorization MUST be absent for initial enrollment and ordinary re-enrollment (SUP §5)"))
	case errors.Is(err, identity.ErrReplacementNotAuthorized):
		WriteProblem(w, r, errpkg.ReplacementNotAuthorized.Problem(err.Error()))
	case errors.Is(err, identity.ErrKeyRotationNotPermitted):
		WriteProblem(w, r, errpkg.KeyRotationNotPermitted.Problem(err.Error()))
	case errors.Is(err, identity.ErrNewLDIForbidden):
		WriteProblem(w, r, errpkg.New(http.StatusForbidden, "Forbidden").
			WithDetail("Trust Domain policy forbids creating a new identity for this bootstrap subject"))
	default:
		WriteProblem(w, r, errpkg.New(http.StatusInternalServerError, "Internal Server Error").
			WithDetail("enrollment failed: "+err.Error()))
	}
}

func derChainToPEM(chain [][]byte) []string {
	out := make([]string, 0, len(chain))
	for _, der := range chain {
		out = append(out, string(pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: der,
		})))
	}
	return out
}
