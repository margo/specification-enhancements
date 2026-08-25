package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	"github.com/margo/miaf-poc/pkg/mis/identity"
)

// RevocationsConfig bundles dependencies for the revocations handler.
type RevocationsConfig struct {
	Queue identity.RevocationQueue
	// Now overrides the clock source. Zero value uses time.Now.
	Now func() time.Time
}

// NewRevocationsHandler builds the GET /api/v1/revocations handler.
func NewRevocationsHandler(cfg RevocationsConfig) http.Handler {
	if cfg.Queue == nil {
		panic("http: RevocationsConfig.Queue is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleRevocations(w, r, cfg)
	})
}

func handleRevocations(w http.ResponseWriter, r *http.Request, cfg RevocationsConfig) {
	now := cfg.Now()

	entries, lastUpdated, err := cfg.Queue.List(r.Context(), now)
	if err != nil {
		WriteProblem(w, r, errpkg.RevocationFormat.Problem("revocation list unavailable: "+err.Error()))
		return
	}

	dto := common.RevocationListDTO{
		RevokedSVIDs: make([]common.RevokedSVIDEntry, 0, len(entries)),
	}
	for _, e := range entries {
		dto.RevokedSVIDs = append(dto.RevokedSVIDs, common.RevokedSVIDEntry{
			CertFingerprintSHA256: e.FingerprintHex,
			SerialNumber:          e.SerialHex,
			RevokedAt:             e.RevokedAt.UTC().Format(time.RFC3339),
		})
	}
	if !lastUpdated.IsZero() {
		dto.LastUpdated = lastUpdated.UTC().Format(time.RFC3339)
	}

	body, err := json.Marshal(&dto)
	if err != nil {
		WriteProblem(w, r, errpkg.RevocationFormat.Problem("marshal: "+err.Error()))
		return
	}

	etag := weakETag(body)
	var lastModifiedHeader string
	if !lastUpdated.IsZero() {
		lastModifiedHeader = lastUpdated.UTC().Format(http.TimeFormat)
	}

	// Conditional: If-None-Match (preferred) => If-Modified-Since.
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if matchesETag(inm, etag) {
			w.Header().Set("ETag", etag)
			if lastModifiedHeader != "" {
				w.Header().Set("Last-Modified", lastModifiedHeader)
			}
			w.WriteHeader(http.StatusNotModified)
			return
		}
	} else if ims := r.Header.Get("If-Modified-Since"); ims != "" && !lastUpdated.IsZero() {
		if t, err := http.ParseTime(ims); err == nil && !lastUpdated.After(t) {
			w.Header().Set("ETag", etag)
			w.Header().Set("Last-Modified", lastModifiedHeader)
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	if lastModifiedHeader != "" {
		w.Header().Set("Last-Modified", lastModifiedHeader)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = bytes.NewReader(body).WriteTo(w)
}
