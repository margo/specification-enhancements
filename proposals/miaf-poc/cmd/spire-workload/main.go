// Command spire-workload is the SPIFFE-native mTLS echo server used to demonstrate
// interoperability of MIS-issued SVIDs with with SPIRE
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	spireworkload "github.com/margo/miaf-poc/pkg/spire-workload"
)

// Authorize any Margo device in the trust domain
func authorizeMargoDeviceIn(td spiffeid.TrustDomain) tlsconfig.Authorizer {
	return tlsconfig.AdaptMatcher(spiffeid.Matcher(func(actual spiffeid.ID) error {
		if !actual.MemberOf(td) || !strings.HasPrefix(actual.Path(), "/margo/device/") {
			return fmt.Errorf("%q is not a Margo device", actual)
		}
		return nil
	}))
}

func main() {
	listenAddr := envOr("SPIRE_WORKLOAD_LISTEN", ":8444")
	trustDomainStr := envOr("SPIRE_WORKLOAD_TRUST_DOMAIN", "margo.example.com")
	workloadAPI := envOr("SPIRE_AGENT_SOCKET", "unix:///run/spire/agent-socket/api.sock")

	td, err := spiffeid.TrustDomainFromString(trustDomainStr)
	if err != nil {
		log.Fatalf("bad trust domain: %v", err)
	}

	log.Printf("spire-workload starting (listen=%s trust-domain=%s workload-api=%s)",
		listenAddr, trustDomainStr, workloadAPI)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Printf("connecting to SPIRE workload API at %s ...", workloadAPI)
	src, err := workloadapi.NewX509Source(ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(workloadAPI)))
	if err != nil {
		log.Fatalf("NewX509Source: %v", err)
	}
	defer src.Close()

	svid, err := src.GetX509SVID()
	if err != nil {
		log.Fatalf("GetX509SVID: %v", err)
	}
	log.Printf("spire-workload SVID: %s (serial %s)", svid.ID, svid.Certificates[0].SerialNumber)

	listener, err := spiffetls.ListenWithMode(ctx, "tcp", listenAddr,
		spiffetls.MTLSServerWithSource(authorizeMargoDeviceIn(td), src))
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	log.Printf("spire-workload listening on %s for peers in trust domain %q", listenAddr, td)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("accept: %v", err)
			continue
		}
		go serve(conn, svid.ID.String())
	}
}

func serve(conn net.Conn, serverID string) {
	defer conn.Close()

	// In TLS 1.3 the server-side handshake (including client cert delivery)
	// may not be complete until the first read. Force it before inspecting peer.
	if h, ok := conn.(interface{ Handshake() error }); ok {
		if err := h.Handshake(); err != nil {
			log.Printf("Handshake: %v", err)
			return
		}
	}
	peerID, err := spiffetls.PeerIDFromConn(conn)
	if err != nil {
		log.Printf("PeerIDFromConn: %v", err)
		return
	}
	if err := spireworkload.HandleConnection(conn, conn, serverID, peerID.String()); err != nil {
		log.Printf("handler: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
