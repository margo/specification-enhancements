package renew

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/csr"
)

// Request bundles the inputs required to perform renewal.
type Request struct {
	Client        *http.Client
	MISBaseURL    string
	SPIFFEID      string
	DevicePrivKey crypto.PrivateKey
	BearerToken   string
}

// Result carries the MIS response. Non-2xx is reported via HTTPStatus +
// Problem; the function returns a non-nil error only on transport failures.
type Result struct {
	HTTPStatus int
	RetryAfter string                       // raw header value; manager parses
	Problem    common.ProblemDetails        // populated on non-2xx when body parses
	Response   common.EnrollmentResponseDTO // populated on 2xx
}

// Renew posts a new CSR to /api/v1/identities/{spiffeIdEncoded}/renewal.
// When BearerToken is empty, the HTTPS transport is expected to carry the
// device's current SVID as the mTLS client certificate. When BearerToken is
// set, the request is authenticated with Authorization: Bearer <jwt-svid>.
func Renew(ctx context.Context, req Request) (*Result, error) {
	if req.Client == nil {
		return nil, fmt.Errorf("renew: Request.Client is nil")
	}
	if req.SPIFFEID == "" {
		return nil, fmt.Errorf("renew: Request.SPIFFEID is empty")
	}
	if req.DevicePrivKey == nil {
		return nil, fmt.Errorf("renew: Request.DevicePrivKey is nil")
	}
	csrB64, err := csr.Build(req.DevicePrivKey)
	if err != nil {
		return nil, err
	}
	body := common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: csrB64},
	}
	buf, err := json.Marshal(&body)
	if err != nil {
		return nil, err
	}
	url := RenewalURL(req.MISBaseURL, req.SPIFFEID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.BearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.BearerToken)
	}

	resp, err := req.Client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	out := &Result{
		HTTPStatus: resp.StatusCode,
		RetryAfter: resp.Header.Get("Retry-After"),
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var decoded common.EnrollmentResponseDTO
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return nil, fmt.Errorf("decode 2xx response: %w", err)
		}
		out.Response = decoded
		return out, nil
	}
	// Best-effort Problem Details parse on non-2xx.
	_ = json.Unmarshal(respBody, &out.Problem)
	return out, nil
}

// RenewalURL returns the MIS renewal endpoint URL for the given SPIFFE ID.
func RenewalURL(misBaseURL, spiffeID string) string {
	enc := common.EncodeSPIFFEID(spiffeID)
	return strings.TrimRight(misBaseURL, "/") + "/api/v1/identities/" + enc + "/renewal"
}
