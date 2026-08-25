package jwtexchange

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ErrInvalidAudience is the sentinel mapped to 400 invalid-audience by the
// HTTP handler.
var ErrInvalidAudience = errors.New("jwtexchange: invalid audience")

// ValidateAudience accepts a non-empty slice of non-empty strings, each parsing
// successfully under url.Parse. It does not require absolute URLs (URN-style
// identifiers are common in service catalogues), only that each element is
// well-formed enough for url.Parse.
func ValidateAudience(aud []string) error {
	if len(aud) == 0 {
		return fmt.Errorf("%w: aud array missing or empty", ErrInvalidAudience)
	}
	for i, a := range aud {
		if strings.TrimSpace(a) == "" {
			return fmt.Errorf("%w: aud[%d] is empty", ErrInvalidAudience, i)
		}
		if _, err := url.Parse(a); err != nil {
			return fmt.Errorf("%w: aud[%d] = %q: %v", ErrInvalidAudience, i, a, err)
		}
	}
	return nil
}
