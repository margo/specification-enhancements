package spireworkload_test

import (
	"bytes"
	"encoding/json"
	"testing"

	spireworkload "github.com/margo/miaf-poc/pkg/spire-workload"
)

func TestHandleConnection_EchoesPeerAndServerIDs(t *testing.T) {
	serverID := "spiffe://margo.example.com/spire/workload/echo"
	peerID := "spiffe://margo.example.com/margo/device/abc-123"

	var in bytes.Buffer
	in.WriteString(`{"hello":"world"}` + "\n")

	var out bytes.Buffer
	if err := spireworkload.HandleConnection(&in, &out, serverID, peerID); err != nil {
		t.Fatalf("HandleConnection: %v", err)
	}

	var got struct {
		PeerSPIFFEID   string `json:"peer_spiffe_id"`
		ServerSPIFFEID string `json:"server_spiffe_id"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &got); err != nil {
		t.Fatalf("unmarshal response: %v (%q)", err, out.String())
	}
	if got.PeerSPIFFEID != peerID {
		t.Errorf("peer_spiffe_id = %q, want %q", got.PeerSPIFFEID, peerID)
	}
	if got.ServerSPIFFEID != serverID {
		t.Errorf("server_spiffe_id = %q, want %q", got.ServerSPIFFEID, serverID)
	}
}

func TestHandleConnection_EmptyRequestStillWrites(t *testing.T) {
	var in bytes.Buffer // EOF on first read
	var out bytes.Buffer
	err := spireworkload.HandleConnection(&in, &out, "spiffe://margo.example.com/spire/workload/echo", "spiffe://margo.example.com/margo/device/x")
	if err != nil {
		t.Fatalf("HandleConnection: %v", err)
	}
	if out.Len() == 0 {
		t.Errorf("response not written on empty request")
	}
}
