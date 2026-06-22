// Package bundle fetches the SPIFFE Bundle Map advertised in the MIAF
// discovery document.
package bundle

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

// Result captures either the served body (on 200) or the not-modified
// indicator (on 304).
type Result struct {
	Body         []byte
	LastModified string
	NotModified  bool
}

// Fetch performs a GET against trustBundleURI. If ifModifiedSince is
// non-empty, it is sent as the If-Modified-Since request header; a 304
// response is surfaced via Result.NotModified.
func Fetch(ctx context.Context, client *http.Client, trustBundleURI, ifModifiedSince string) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, trustBundleURI, nil)
	if err != nil {
		return nil, err
	}
	if ifModifiedSince != "" {
		req.Header.Set("If-Modified-Since", ifModifiedSince)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", trustBundleURI, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return &Result{Body: body, LastModified: resp.Header.Get("Last-Modified")}, nil
	case http.StatusNotModified:
		return &Result{NotModified: true, LastModified: resp.Header.Get("Last-Modified")}, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bundle map returned %d: %s", resp.StatusCode, string(body))
	}
}
