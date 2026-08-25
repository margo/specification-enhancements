package adminclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
)

// Client POSTs JSON to the MIS admin API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client targeting baseURL (e.g., http://127.0.0.1:8445).
// Trailing slashes are trimmed.
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// CreateEnrollmentToken calls POST /admin/v1/enrollment-tokens.
func (c *Client) CreateEnrollmentToken(ctx context.Context, ttl time.Duration) (*common.CreateEnrollmentTokenResponse, error) {
	body, err := json.Marshal(common.CreateEnrollmentTokenRequest{TTL: ttl.String()})
	if err != nil {
		return nil, err
	}
	var resp common.CreateEnrollmentTokenResponse
	if err := c.post(ctx, "/admin/v1/enrollment-tokens", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateReplacementTicket calls POST /admin/v1/replacement-tickets.
func (c *Client) CreateReplacementTicket(ctx context.Context, spiffeID string, ttl time.Duration) (*common.CreateReplacementTicketResponse, error) {
	body, err := json.Marshal(common.CreateReplacementTicketRequest{SPIFFEID: spiffeID, TTL: ttl.String()})
	if err != nil {
		return nil, err
	}
	var resp common.CreateReplacementTicketResponse
	if err := c.post(ctx, "/admin/v1/replacement-tickets", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Revoke calls POST /admin/v1/revocations.
func (c *Client) Revoke(ctx context.Context, spiffeID string, reason string) (*common.RevokeSPIFFEIDResponse, error) {
	body, err := json.Marshal(common.RevokeSPIFFEIDRequest{SPIFFEID: spiffeID, Reason: reason})
	if err != nil {
		return nil, err
	}
	var resp common.RevokeSPIFFEIDResponse
	if err := c.post(ctx, "/admin/v1/revocations", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) post(ctx context.Context, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("admin %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin %s: %s: %s", path, resp.Status, strings.TrimSpace(string(raw)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
