// Package http holds the MIS HTTP transport layer.
package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
)

type ctxKey int

const requestIDKey ctxKey = iota

// RequestIDFromContext returns the per-request UUID set by WithRequestID, or
// the empty string if none is set.
func RequestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// WithRequestID assigns a fresh UUID4 to each request and puts it on the
// context. Handlers can read it with RequestIDFromContext.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AccessLog emits one slog line per request with method, path, status, and
// latency. It wraps ResponseWriter to observe the status code.
func AccessLog(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http_request",
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Duration("latency", time.Since(start)),
			)
		})
	}
}

// WriteProblem renders an RFC 9457 Problem Details response. If the request
// context carries a request ID (set by WithRequestID), it is attached to the
// response body as the "instance" URI for cross-service correlation, unless
// the caller already provided one.
func WriteProblem(w http.ResponseWriter, r *http.Request, pd *errpkg.ProblemDetails) {
	out := *pd
	if rid := RequestIDFromContext(r.Context()); rid != "" && out.Instance == "" {
		out.Instance = "urn:uuid:" + rid
	}
	w.Header().Set("Content-Type", errpkg.ContentType)
	w.WriteHeader(out.Status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(&out)
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}
