package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
)

// BundleSource returns the current trust-anchor set for the local trust
// domain plus the time at which it was last refreshed. Implementations may
// cache; MIS loads from disk at startup and currently never refreshes.
type BundleSource interface {
	GetTrustAnchors(ctx context.Context) (bundlemap.TrustAnchorSet, time.Time, error)
}

// NewBundleMapHandler serves the SPIFFE Bundle Map resource identified by
// discovery's trust_bundle_uri. Supports conditional requests via
// ETag / If-None-Match (preferred) and Last-Modified / If-Modified-Since.
// maxAge sets the Cache-Control max-age hint; pass 0 to omit the header.
func NewBundleMapHandler(src BundleSource, maxAge time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set, refreshed, err := src.GetTrustAnchors(r.Context())
		if err != nil {
			WriteProblem(w, r, errpkg.New(http.StatusServiceUnavailable, "Bundle Map Unavailable").
				WithDetail("MIS could not retrieve the trust anchor set"))
			return
		}

		// Build the body first so we can derive the ETag before any conditional checks.
		body, err := bundlemap.Build(set)
		if err != nil {
			WriteProblem(w, r, errpkg.New(http.StatusInternalServerError, "Bundle Map Error").
				WithDetail("MIS could not serialize the Bundle Map"))
			return
		}

		etag := weakETag(body)

		// RFC 9110 §13.2.2: If-None-Match takes precedence over If-Modified-Since.
		if inm := r.Header.Get("If-None-Match"); inm != "" {
			if matchesETag(inm, etag) {
				w.Header().Set("ETag", etag)
				if !refreshed.IsZero() {
					w.Header().Set("Last-Modified", refreshed.UTC().Format(http.TimeFormat))
				}
				w.WriteHeader(http.StatusNotModified)
				return
			}
		} else if ims := r.Header.Get("If-Modified-Since"); ims != "" {
			// Conditional request - byte-exact comparison on HTTP-time.
			if t, err := http.ParseTime(ims); err == nil && !refreshed.After(t) {
				w.Header().Set("ETag", etag)
				if !refreshed.IsZero() {
					w.Header().Set("Last-Modified", refreshed.UTC().Format(http.TimeFormat))
				}
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if !refreshed.IsZero() {
			w.Header().Set("Last-Modified", refreshed.UTC().Format(http.TimeFormat))
		}
		w.Header().Set("ETag", etag)
		if secs := int(maxAge.Seconds()); secs > 0 {
			w.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", secs))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}
