package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	errpkg "github.com/margo/miaf-poc/pkg/mis/errors"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestWriteProblem_AboutBlank(t *testing.T) {
	rr := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	mishttp.WriteProblem(rr, r, errpkg.New(401, "Unauthorized").WithDetail("bad cred"))

	if rr.Code != 401 {
		t.Errorf("status = %d, want 401", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != errpkg.ContentType {
		t.Errorf("Content-Type = %q, want %q", ct, errpkg.ContentType)
	}

	var body errpkg.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Type != "about:blank" || body.Status != 401 || body.Detail != "bad cred" {
		t.Errorf("unexpected body: %+v", body)
	}
}

func TestWriteProblem_InstanceFromContext(t *testing.T) {
	rr := httptest.NewRecorder()
	// Simulate middleware placing a request ID on the context.
	h := mishttp.WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mishttp.WriteProblem(w, r, errpkg.UnsupportedBootstrapMethod.Problem("nope"))
	}))
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(context.Background()))

	if rr.Code != 422 {
		t.Errorf("status = %d, want 422", rr.Code)
	}
	var body errpkg.ProblemDetails
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.HasPrefix(body.Instance, "urn:uuid:") {
		t.Errorf("instance = %q, want urn:uuid:* prefix", body.Instance)
	}
}
