package common_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/margo/miaf-poc/pkg/common"
)

func TestRevocationListDTO_JSONRoundTrip(t *testing.T) {
	in := common.RevocationListDTO{
		LastUpdated: "2025-10-25T14:12:31Z",
		RevokedSVIDs: []common.RevokedSVIDEntry{
			{
				CertFingerprintSHA256: "2f4c3d9a7b1e0c6d8f2a1b9c0d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e",
				SerialNumber:          "8F12A4C9D23E41B1",
				RevokedAt:             "2025-10-20T09:33:45Z",
			},
		},
	}
	b, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out common.RevocationListDTO
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.LastUpdated != in.LastUpdated {
		t.Errorf("last_updated = %q, want %q", out.LastUpdated, in.LastUpdated)
	}
	if len(out.RevokedSVIDs) != 1 || out.RevokedSVIDs[0] != in.RevokedSVIDs[0] {
		t.Errorf("RevokedSVIDs mismatch: got %+v", out.RevokedSVIDs)
	}
}

func TestRevocationListDTO_EmptyListIsArrayNotNull(t *testing.T) {
	dto := common.RevocationListDTO{
		LastUpdated:  "",
		RevokedSVIDs: []common.RevokedSVIDEntry{},
	}
	b, err := json.Marshal(&dto)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(b, []byte(`"revoked_svids":[]`)) {
		t.Errorf("expected empty array, got: %s", b)
	}
}

func TestRevokeSPIFFEIDRequest_JSON(t *testing.T) {
	in := common.RevokeSPIFFEIDRequest{SPIFFEID: "spiffe://td/margo/device/abc", Reason: "compromise"}
	b, _ := json.Marshal(&in)
	var out common.RevokeSPIFFEIDRequest
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round trip: got %+v, want %+v", out, in)
	}
}
