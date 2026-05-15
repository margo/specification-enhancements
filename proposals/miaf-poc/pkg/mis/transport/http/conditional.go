package http

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// weakETag computes a weak ETag for body as W/"<16-hex-char prefix of SHA-256>".
// Used by both the Bundle Map and revocation-list handlers.
func weakETag(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf(`W/"%x"`, sum[:8])
}

// matchesETag reports whether the If-None-Match header value matches etag.
// Accepts the wildcard "*" and a comma-separated list of tag values.
func matchesETag(header, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "*" {
		return true
	}
	for _, part := range strings.Split(header, ",") {
		if strings.TrimSpace(part) == etag {
			return true
		}
	}
	return false
}
