package http

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// ErrSPIFFEIDEncoding is returned when the {spiffeIdEncoded} URL-path
// parameter cannot be decoded as a valid SPIFFE ID per SUP §"Common URI
// and Encoding Rules".
var ErrSPIFFEIDEncoding = errors.New("transport: malformed {spiffeIdEncoded} path parameter")

// DecodeSPIFFEIDEncoded reverses common.EncodeSPIFFEID. It rejects any input
// that is not raw base64url (no padding) and any decoded value that does not
// begin with "spiffe://".
func DecodeSPIFFEIDEncoded(encoded string) (string, error) {
	if encoded == "" {
		return "", fmt.Errorf("%w: empty", ErrSPIFFEIDEncoding)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSPIFFEIDEncoding, err)
	}
	s := string(raw)
	if !strings.HasPrefix(s, "spiffe://") {
		return "", fmt.Errorf("%w: not a SPIFFE URI", ErrSPIFFEIDEncoding)
	}
	return s, nil
}
