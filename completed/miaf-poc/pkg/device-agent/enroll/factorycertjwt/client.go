package factorycertjwt

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/csr"
)

// Request bundles the inputs required for enrollment via factory-cert-jwt.
//
// The Client must be configured for server-authenticated HTTPS (e.g. via
// httpclient.New); no client certificate is required.
type Request struct {
	Client                   *http.Client
	MISBaseURL               string
	FactoryCerts             []*x509.Certificate
	FactoryKey               crypto.Signer
	DevicePrivKey            crypto.PrivateKey
	ReplacementAuthorization *common.ReplacementAuthorizationDTO
}

// Result carries the MIS response. Non-2xx is reported via HTTPStatus +
// Problem; the function returns a non-nil error only on transport failures.
type Result struct {
	HTTPStatus int
	RetryAfter string
	Problem    common.ProblemDetails
	Response   common.EnrollmentResponseDTO // populated on 2xx
}

// Enroll performs POST <MISBaseURL>/api/v1/identities with a freshly-built
// CSR and the factory-cert-jwt bootstrap credential.
func Enroll(ctx context.Context, req Request) (*Result, error) {
	if req.Client == nil {
		return nil, fmt.Errorf("factorycertjwt: Request.Client is nil")
	}
	if len(req.FactoryCerts) == 0 {
		return nil, fmt.Errorf("factorycertjwt: Request.FactoryCerts is empty")
	}
	if req.FactoryKey == nil {
		return nil, fmt.Errorf("factorycertjwt: Request.FactoryKey is nil")
	}
	if req.DevicePrivKey == nil {
		return nil, fmt.Errorf("factorycertjwt: Request.DevicePrivKey is nil")
	}

	url := strings.TrimRight(req.MISBaseURL, "/") + "/api/v1/identities"
	assertion, err := BuildAssertion(AssertionInput{
		FactoryCerts: req.FactoryCerts,
		FactoryKey:   req.FactoryKey,
		Audience:     url,
	})
	if err != nil {
		return nil, err
	}

	csrB64, err := csr.Build(req.DevicePrivKey)
	if err != nil {
		return nil, err
	}

	body := common.EnrollmentRequestDTO{
		SVIDProfileURI: common.SVIDProfileURIX509V1,
		SVIDRequest:    common.SVIDRequestX509DTO{CSR: csrB64},
		BootstrapCredential: common.BootstrapCredentialDTO{
			Method: common.BootstrapMethodFactoryCertJWT,
			Proof:  &common.BootstrapProof{Assertion: assertion},
		},
	}
	if req.ReplacementAuthorization != nil {
		body.ReplacementAuthorization = req.ReplacementAuthorization
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

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("factorycertjwt: read response body: %w", err)
	}
	out := &Result{
		HTTPStatus: resp.StatusCode,
		RetryAfter: resp.Header.Get("Retry-After"),
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var decoded common.EnrollmentResponseDTO
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return nil, fmt.Errorf("factorycertjwt: decode 2xx response: %w", err)
		}
		out.Response = decoded
		return out, nil
	}
	// Best-effort Problem Details parse on non-2xx.
	_ = json.Unmarshal(respBody, &out.Problem)
	return out, nil
}
