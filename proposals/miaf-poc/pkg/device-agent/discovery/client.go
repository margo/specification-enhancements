// Package discovery fetches the MIAF discovery document from a MIS base URL.
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/margo/miaf-poc/pkg/common"
)

// Fetch performs GET misBaseURL/.well-known/margo and decodes the response.
func Fetch(ctx context.Context, client *http.Client, misBaseURL string) (*common.DiscoveryResponseDTO, error) {
	u := strings.TrimRight(misBaseURL, "/") + "/.well-known/margo"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery returned %d: %s", resp.StatusCode, string(body))
	}

	var out common.DiscoveryResponseDTO
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode discovery: %w", err)
	}
	return &out, nil
}
