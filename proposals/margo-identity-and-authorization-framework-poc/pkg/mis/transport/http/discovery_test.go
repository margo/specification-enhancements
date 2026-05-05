package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

func TestDiscoveryHandler_ReturnsSUPExample(t *testing.T) {
	doc := &common.DiscoveryResponseDTO{
		TrustDomain:                 "northstar-ida.com",
		TrustBundleURI:              "https://mis.northstar-ida.com/.well-known/spiffe/bundle.json",
		MargoIdentityServiceBaseURI: "https://mis.northstar-ida.com",
		SupportedBootstrapMethods: []string{
			common.BootstrapMethodFactoryCertMTLS,
			common.BootstrapMethodFactoryCertJWT,
			common.BootstrapMethodFDO,
			common.BootstrapMethodEnrollmentToken,
		},
		SVIDProfilesSupported: []common.SVIDProfileDescriptor{
			{Type: common.SVIDProfileTypeX509, Versions: []string{common.SVIDProfileURIX509V1}},
			{Type: common.SVIDProfileTypeJWT, Versions: []string{common.SVIDProfileURIJWTV1}},
		},
		RecommendedSVIDProfileURI: common.SVIDProfileURIX509V1,
	}

	ts := httptest.NewServer(mishttp.NewDiscoveryHandler(doc))
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	body, _ := io.ReadAll(resp.Body)
	var got common.DiscoveryResponseDTO
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TrustDomain != doc.TrustDomain || got.MargoIdentityServiceBaseURI != doc.MargoIdentityServiceBaseURI {
		t.Errorf("round trip changed content:\ngot:  %+v\nwant: %+v", got, *doc)
	}
	if len(got.SupportedBootstrapMethods) != 4 {
		t.Errorf("supported_bootstrap_methods len = %d, want 4", len(got.SupportedBootstrapMethods))
	}
}
