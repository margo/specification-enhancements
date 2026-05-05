package jwtsvid

import (
	"bytes"
	"context"
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
)

// Mode selects the authentication mechanism.
type Mode int

const (
	// ModeMTLS sends the request without a client_assertion; Request.Client
	// must be configured for mTLS using the device's current SVID.
	ModeMTLS Mode = iota
	// ModeClientAssertion includes a client_assertion JWT in the body and
	// uses Request.Client for plain HTTPS (no mTLS).
	ModeClientAssertion
)

// Request bundles the inputs required to perform JWT SVID exchange.
type Request struct {
	Client     *http.Client
	MISBaseURL string
	SPIFFEID   string
	Audiences  []string
	TTL        time.Duration
	Mode       Mode
	// SigningKey is required when Mode == ModeClientAssertion. It is the
	// device's current SVID private key.
	SigningKey crypto.Signer
	// Now overrides the clock for assertion iat/exp. Zero uses time.Now.
	Now func() time.Time
}

// Result carries the MIS response. Non-2xx is reported via HTTPStatus +
// Problem; the function returns a non-nil error only on transport failures.
type Result struct {
	HTTPStatus int
	RetryAfter string
	Problem    common.ProblemDetails
	Response   common.JWTSVIDExchangeResponseDTO
}

// Exchange posts a request to /api/v1/identities/{spiffeIdEncoded}/jwt-svid.
func Exchange(ctx context.Context, req Request) (*Result, error) {
	if req.Client == nil {
		return nil, fmt.Errorf("jwtsvid: Request.Client is nil")
	}
	if req.SPIFFEID == "" {
		return nil, fmt.Errorf("jwtsvid: Request.SPIFFEID is empty")
	}
	if len(req.Audiences) == 0 {
		return nil, fmt.Errorf("jwtsvid: Request.Audiences is empty")
	}
	if req.Now == nil {
		req.Now = time.Now
	}

	enc := common.EncodeSPIFFEID(req.SPIFFEID)
	url := strings.TrimRight(req.MISBaseURL, "/") + "/api/v1/identities/" + enc + "/jwt-svid"

	body := common.JWTSVIDExchangeRequestDTO{
		Audience:   req.Audiences,
		TTLSeconds: int(req.TTL.Seconds()),
	}
	if req.Mode == ModeClientAssertion {
		if req.SigningKey == nil {
			return nil, fmt.Errorf("jwtsvid: SigningKey required for ModeClientAssertion")
		}
		assertion, err := BuildClientAssertion(req.SigningKey, req.SPIFFEID, url, req.Now())
		if err != nil {
			return nil, err
		}
		body.ClientAssertion = assertion
	}
	buf, err := json.Marshal(&body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		var decoded common.JWTSVIDExchangeResponseDTO
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
		out.Response = decoded
		return out, nil
	}
	_ = json.Unmarshal(respBody, &out.Problem)
	return out, nil
}
