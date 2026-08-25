package interop_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/url"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/margo/miaf-poc/pkg/device-agent/interop"
	spireworkload "github.com/margo/miaf-poc/pkg/spire-workload"
)

func TestDial_ReturnsBothSPIFFEIDs(t *testing.T) {
	td := spiffeid.RequireTrustDomainFromString("margo.example.com")

	root, rootKey := makeCA(t, "test-root")
	clientSVID := makeLeaf(t, td, "/margo/device/abc", root, rootKey)
	serverSVID := makeLeaf(t, td, "/spire/workload/echo", root, rootKey)
	bundleSrc := x509bundle.FromX509Authorities(td, []*x509.Certificate{root})

	ln, err := spiffetls.ListenWithMode(context.Background(), "tcp", "127.0.0.1:0",
		spiffetls.MTLSServerWithRawConfig(tlsconfig.AuthorizeMemberOf(td), serverSVID, bundleSrc))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Force TLS 1.3 handshake completion before reading peer certs.
		if h, ok := conn.(interface{ Handshake() error }); ok {
			if err := h.Handshake(); err != nil {
				return
			}
		}
		peerID, _ := spiffetls.PeerIDFromConn(conn)
		_ = spireworkload.HandleConnection(conn, conn, serverSVID.ID.String(), peerID.String())
	}()

	resp, err := interop.Dial(context.Background(), interop.DialInput{
		ServerAddr:   ln.Addr().String(),
		TrustDomain:  td,
		ClientSVID:   clientSVID,
		BundleSource: bundleSrc,
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if resp.PeerSPIFFEID != serverSVID.ID.String() {
		t.Errorf("peer = %q, want %q", resp.PeerSPIFFEID, serverSVID.ID.String())
	}
	if resp.ServerSPIFFEID != serverSVID.ID.String() {
		t.Errorf("server_spiffe_id = %q, want %q", resp.ServerSPIFFEID, serverSVID.ID.String())
	}
}

func makeCA(t *testing.T, cn string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	cert, _ := x509.ParseCertificate(der)
	return cert, key
}

func makeLeaf(t *testing.T, td spiffeid.TrustDomain, path string, ca *x509.Certificate, caKey *ecdsa.PrivateKey) *x509svid.SVID {
	t.Helper()
	leafKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	u := &url.URL{Scheme: "spiffe", Host: td.String(), Path: path}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "leaf"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{u},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("sign leaf: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	svid, err := x509svid.Parse(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("x509svid.Parse: %v", err)
	}
	return svid
}
