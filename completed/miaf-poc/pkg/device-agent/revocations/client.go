package revocations

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/margo/miaf-poc/pkg/common"
)

// Result captures the response of a Fetch.
type Result struct {
	HTTPStatus int
	RetryAfter string
	Problem    common.ProblemDetails
	// NotModified is true when the server returned 304.
	NotModified bool
	// List is populated only when NotModified is false.
	List common.RevocationListDTO
	// ETag and LastModified are the validators returned by the server.
	// Callers persist these to enable a subsequent conditional GET.
	ETag         string
	LastModified string
}

// ContainsFingerprint reports whether fp (lowercase-hex SHA-256) appears in
// the list. Primary matching identifier per SUP §"Revocation List Endpoint"
// ("Consumers MUST use this value as the primary identifier when checking
// revocation status").
func (r *Result) ContainsFingerprint(fp string) bool {
	for _, e := range r.List.RevokedSVIDs {
		if e.CertFingerprintSHA256 == fp {
			return true
		}
	}
	return false
}

// Fetch performs a GET against revocationsURI. When etag is non-empty it is
// sent as If-None-Match; when lastModified is non-empty it is sent as
// If-Modified-Since (If-None-Match takes precedence per RFC 9110).
func Fetch(ctx context.Context, client *http.Client, revocationsURI, etag, lastModified string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, revocationsURI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	} else if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", revocationsURI, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
		var list common.RevocationListDTO
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, fmt.Errorf("parse revocation list: %w", err)
		}
		return &Result{
			HTTPStatus:   http.StatusOK,
			List:         list,
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
		}, nil
	case http.StatusNotModified:
		return &Result{
			HTTPStatus:   http.StatusNotModified,
			NotModified:  true,
			ETag:         resp.Header.Get("ETag"),
			LastModified: resp.Header.Get("Last-Modified"),
		}, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		out := &Result{
			HTTPStatus: resp.StatusCode,
			RetryAfter: resp.Header.Get("Retry-After"),
		}
		_ = json.Unmarshal(body, &out.Problem)
		return out, nil
	}
}
