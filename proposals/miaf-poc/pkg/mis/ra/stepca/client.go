package stepca

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/google/uuid"
)

// ClientConfig parametrises NewClient.
type ClientConfig struct {
	BaseURL     string
	RootPool    *x509.CertPool
	Credential  *Credential
	HTTPTimeout time.Duration
}

// Client is a thin Step CA RA client.
type Client struct {
	baseURL     string
	provisioner string
	keyID       string
	signer      crypto.Signer
	http        *http.Client
}

// NewClient constructs a Client.
func NewClient(cfg ClientConfig) (*Client, error) {
	if cfg.Credential == nil {
		return nil, fmt.Errorf("stepca: NewClient: Credential is required")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("stepca: NewClient: BaseURL is required")
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 15 * time.Second
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: cfg.RootPool, MinVersion: tls.VersionTLS12},
	}
	return &Client{
		baseURL:     cfg.BaseURL,
		provisioner: cfg.Credential.ProvisionerName,
		keyID:       cfg.Credential.KeyID,
		signer:      cfg.Credential.SigningKey,
		http:        &http.Client{Timeout: cfg.HTTPTimeout, Transport: tr},
	}, nil
}

// SignRequest is the input to Client.Sign.
type SignRequest struct {
	SpiffeID string
	CSRDER   []byte
	TTL      time.Duration
}

// signResponse matches Step CA's /1.0/sign JSON body.
type signResponse struct {
	Certificate string   `json:"crt"`
	CA          string   `json:"ca"`
	CertChain   []string `json:"certChain"`
}

// Sign POSTs to /1.0/sign and returns the chain as DER plus the leaf NotAfter.
func (c *Client) Sign(ctx context.Context, req SignRequest) ([][]byte, time.Time, error) {
	// Step CA 0.30 requires a non-empty sub and validates it against the CSR's
	// CommonName. Extract the CN from the CSR and use it as the OTT subject so
	// the CN-matching check passes. The X.509 template controls the issued
	// cert's subject and SANs via user.sans - the CSR CN is not reflected in
	// the certificate. When the CSR has no CN, fall back to the SPIFFE ID.
	csrForSub, err := x509.ParseCertificateRequest(req.CSRDER)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: parse CSR for OTT subject: %w", err)
	}
	ottSubject := csrForSub.Subject.CommonName
	if ottSubject == "" {
		ottSubject = req.SpiffeID
	}

	ott, err := c.mintOTT(ottSubject, req.SpiffeID)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: mint OTT: %w", err)
	}

	csrPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: req.CSRDER})
	body, err := json.Marshal(map[string]any{
		"ott":      ott,
		"csr":      string(csrPEM),
		"notAfter": time.Now().Add(req.TTL).UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: marshal sign request: %w", err)
	}

	endpoint, err := url.JoinPath(c.baseURL, "/1.0/sign")
	if err != nil {
		return nil, time.Time{}, err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, time.Time{}, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(hreq)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return nil, time.Time{}, fmt.Errorf("stepca: /1.0/sign returned %s: %s", resp.Status, string(slurp))
	}

	var sr signResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: decode sign response: %w", err)
	}
	chain, err := decodePEMChain(sr.CertChain, sr.Certificate, sr.CA)
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(chain) == 0 {
		return nil, time.Time{}, fmt.Errorf("stepca: empty chain in response")
	}
	leaf, err := x509.ParseCertificate(chain[0])
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("stepca: parse leaf: %w", err)
	}
	return chain, leaf.NotAfter, nil
}

// decodePEMChain prefers certChain (ordered leaf => root); falls back to
// crt + ca if certChain is absent (older Step CA versions).
func decodePEMChain(certChain []string, crt, ca string) ([][]byte, error) {
	if len(certChain) > 0 {
		out := make([][]byte, 0, len(certChain))
		for i, p := range certChain {
			der, err := pemToDER(p)
			if err != nil {
				return nil, fmt.Errorf("stepca: certChain[%d]: %w", i, err)
			}
			out = append(out, der)
		}
		return out, nil
	}
	if crt == "" {
		return nil, fmt.Errorf("stepca: response has no crt and no certChain")
	}
	leafDER, err := pemToDER(crt)
	if err != nil {
		return nil, fmt.Errorf("stepca: crt: %w", err)
	}
	out := [][]byte{leafDER}
	if ca != "" {
		caDER, err := pemToDER(ca)
		if err != nil {
			return nil, fmt.Errorf("stepca: ca: %w", err)
		}
		out = append(out, caDER)
	}
	return out, nil
}

func pemToDER(p string) ([]byte, error) {
	block, _ := pem.Decode([]byte(p))
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	return block.Bytes, nil
}

// mintOTT builds a JWT signed by the provisioner JWK. The top-level "sans"
// claim carries the authoritative URI SAN; step-ca converts it via CreateSANs
// (auto-detecting the spiffe:// URI scheme) and the X.509 template reads .SANs
// rather than anything from the CSR. Cert validity is controlled via the
// "notAfter" field in the /1.0/sign request body, not through this token.
func (c *Client) mintOTT(subject, spiffeID string) (string, error) {
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.ES256, Key: c.signer},
		(&jose.SignerOptions{}).
			WithType("JWT").
			WithHeader("kid", c.keyID),
	)
	if err != nil {
		return "", err
	}

	now := time.Now()
	claims := jwt.Claims{
		ID:        uuid.NewString(),
		Issuer:    c.provisioner,
		Subject:   subject,
		Audience:  jwt.Audience{c.baseURL + "/1.0/sign"},
		NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
		IssuedAt:  jwt.NewNumericDate(now),
		Expiry:    jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}

	// "sans" is the JWK provisioner's designed mechanism for overriding SANs.
	// step-ca calls CreateSANs([]string{spiffeID}) which auto-detects the
	// spiffe:// URI scheme, yielding [{type:"uri",value:"spiffe://..."}].
	// The X.509 template then reads .SANs (already typed) rather than raw
	// Insecure.User data, so toJson produces the correct {type,value} format.
	return jwt.Signed(signer).Claims(claims).Claims(map[string]any{"sans": []string{spiffeID}}).Serialize()
}

// NewCredentialForTest is a testing helper that constructs a Credential without
// going through the JWE decryption path. External callers MUST NOT use this.
func NewCredentialForTest(name, kid string, signer crypto.Signer) *Credential {
	return &Credential{ProvisionerName: name, KeyID: kid, SigningKey: signer}
}
