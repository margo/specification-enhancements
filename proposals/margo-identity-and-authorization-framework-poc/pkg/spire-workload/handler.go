// Package spireworkload implements the workload-side echo protocol used by
// the MIAF PoC's SPIFFE interop demonstration.
package spireworkload

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
)

// Response is the JSON payload the echo handler writes back to the peer.
type Response struct {
	PeerSPIFFEID   string `json:"peer_spiffe_id"`
	ServerSPIFFEID string `json:"server_spiffe_id"`
}

// HandleConnection reads a single line of JSON (or EOF) from in, then writes
// a Response on out identifying both sides of the mTLS connection.
func HandleConnection(in io.Reader, out io.Writer, serverID, peerID string) error {
	r := bufio.NewReader(in)
	if _, err := r.ReadString('\n'); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	resp := Response{PeerSPIFFEID: peerID, ServerSPIFFEID: serverID}
	return json.NewEncoder(out).Encode(resp)
}
