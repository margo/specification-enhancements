package interop

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// DialInput carries the materials the interop-call subcommand assembles
// before dialing the workload.
type DialInput struct {
	ServerAddr   string
	TrustDomain  spiffeid.TrustDomain
	ClientSVID   *x509svid.SVID
	BundleSource *x509bundle.Bundle
}

// Response mirrors the JSON shape the spire-workload handler emits.
type Response struct {
	PeerSPIFFEID   string `json:"peer_spiffe_id"`
	ServerSPIFFEID string `json:"server_spiffe_id"`
}

// Dial performs a mutual mTLS connection to in.ServerAddr using in.ClientSVID
// and a trust pool sourced from in.BundleSource. The peer is authorized if
// its SPIFFE ID is in in.TrustDomain.
//
// PeerSPIFFEID in the returned Response is extracted from the TLS peer
// certificate (the workload's identity as verified by mTLS). ServerSPIFFEID
// is the workload's self-reported identity from the echo JSON body. In a
// correctly configured stack the two agree.
func Dial(ctx context.Context, in DialInput) (*Response, error) {
	conn, err := spiffetls.DialWithMode(ctx, "tcp", in.ServerAddr,
		spiffetls.MTLSClientWithRawConfig(tlsconfig.AuthorizeMemberOf(in.TrustDomain), in.ClientSVID, in.BundleSource))
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", in.ServerAddr, err)
	}
	defer conn.Close()

	// tls.Dial already called Handshake; peer cert is available.
	peerID, err := spiffetls.PeerIDFromConn(conn)
	if err != nil {
		return nil, fmt.Errorf("peer SPIFFE ID from TLS cert: %w", err)
	}

	if _, err := fmt.Fprintln(conn, `{"from":"device-agent"}`); err != nil {
		return nil, fmt.Errorf("write ping: %w", err)
	}

	var echo struct {
		ServerSPIFFEID string `json:"server_spiffe_id"`
	}
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&echo); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &Response{
		PeerSPIFFEID:   peerID.String(),
		ServerSPIFFEID: echo.ServerSPIFFEID,
	}, nil
}
