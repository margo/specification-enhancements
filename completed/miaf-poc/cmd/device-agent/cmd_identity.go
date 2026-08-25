// Device identity lifecycle commands: enroll (all bootstrap methods), renew, and check-revocations.
package main

import (
	"context"
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/device-agent/discovery"
	"github.com/margo/miaf-poc/pkg/device-agent/enroll/enrollmenttoken"
	"github.com/margo/miaf-poc/pkg/device-agent/enroll/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/device-agent/enroll/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
	"github.com/margo/miaf-poc/pkg/device-agent/renew"
	"github.com/margo/miaf-poc/pkg/device-agent/revocations"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

func enrollFactoryCertMTLS(ctx context.Context, cmd *cli.Command) error {
	misBaseURL := cmd.Root().String("mis-base-url")
	trustAnchor := cmd.Root().String("mis-trust-anchor")
	enrollCmd := cmd.Lineage()[1]
	repl := replacementAuthorizationForTicket(enrollCmd.String("replacement-ticket"))

	client, err := httpclient.NewMTLS(trustAnchor, cmd.String("factory-cert"), cmd.String("factory-key"))
	if err != nil {
		return err
	}

	alg := keygen.Algorithm(enrollCmd.String("svid-key-alg"))
	key, err := keygen.Generate(alg)
	if err != nil {
		return err
	}

	res, err := factorycertmtls.Enroll(ctx, factorycertmtls.Request{
		Client:                   client,
		MISBaseURL:               misBaseURL,
		DevicePrivKey:            key,
		ReplacementAuthorization: repl,
	})
	if err != nil {
		return err
	}
	if res.HTTPStatus < 200 || res.HTTPStatus >= 300 {
		return fmt.Errorf("enrollment %d: %s", res.HTTPStatus, res.Problem.Title)
	}
	return persistEnrollmentResult(enrollCmd, res.HTTPStatus, res.Response.SVID.CertificateChainPEM, key)
}

func enrollFactoryCertJWT(ctx context.Context, cmd *cli.Command) error {
	misBaseURL := cmd.Root().String("mis-base-url")
	trustAnchor := cmd.Root().String("mis-trust-anchor")
	enrollCmd := cmd.Lineage()[1]
	repl := replacementAuthorizationForTicket(enrollCmd.String("replacement-ticket"))

	factoryCerts, err := loadFactoryChainPEM(cmd.String("factory-cert"))
	if err != nil {
		return fmt.Errorf("factory cert: %w", err)
	}
	factoryKey, err := loadFactoryKeyPEM(cmd.String("factory-key"))
	if err != nil {
		return fmt.Errorf("factory key: %w", err)
	}

	client, err := httpclient.New(trustAnchor)
	if err != nil {
		return err
	}

	alg := keygen.Algorithm(enrollCmd.String("svid-key-alg"))
	devKey, err := keygen.Generate(alg)
	if err != nil {
		return err
	}

	res, err := factorycertjwt.Enroll(ctx, factorycertjwt.Request{
		Client:                   client,
		MISBaseURL:               misBaseURL,
		FactoryCerts:             factoryCerts,
		FactoryKey:               factoryKey,
		DevicePrivKey:            devKey,
		ReplacementAuthorization: repl,
	})
	if err != nil {
		return err
	}
	if res.HTTPStatus < 200 || res.HTTPStatus >= 300 {
		return fmt.Errorf("enrollment %d: %s", res.HTTPStatus, res.Problem.Title)
	}
	return persistEnrollmentResult(enrollCmd, res.HTTPStatus, res.Response.SVID.CertificateChainPEM, devKey)
}

func enrollEnrollmentToken(ctx context.Context, cmd *cli.Command) error {
	misBaseURL := cmd.Root().String("mis-base-url")
	trustAnchor := cmd.Root().String("mis-trust-anchor")
	enrollCmd := cmd.Lineage()[1]
	repl := replacementAuthorizationForTicket(enrollCmd.String("replacement-ticket"))

	token, err := resolveToken(cmd.String("token"), cmd.String("token-file"))
	if err != nil {
		return err
	}

	client, err := httpclient.New(trustAnchor)
	if err != nil {
		return err
	}

	alg := keygen.Algorithm(enrollCmd.String("svid-key-alg"))
	devKey, err := keygen.Generate(alg)
	if err != nil {
		return err
	}

	res, err := enrollmenttoken.Enroll(ctx, enrollmenttoken.Request{
		Client:                   client,
		MISBaseURL:               misBaseURL,
		Token:                    token,
		DevicePrivKey:            devKey,
		ReplacementAuthorization: repl,
	})
	if err != nil {
		return err
	}
	if res.HTTPStatus < 200 || res.HTTPStatus >= 300 {
		return fmt.Errorf("enrollment %d: %s", res.HTTPStatus, res.Problem.Title)
	}
	return persistEnrollmentResult(enrollCmd, res.HTTPStatus, res.Response.SVID.CertificateChainPEM, devKey)
}

func replacementAuthorizationForTicket(ticket string) *common.ReplacementAuthorizationDTO {
	if ticket == "" {
		return nil
	}
	return &common.ReplacementAuthorizationDTO{
		Method: common.ReplacementAuthMethodOperatorTicket,
		Proof:  common.ReplacementAuthorizationProof{Ticket: ticket},
	}
}

func renewSVID(ctx context.Context, cmd *cli.Command) error {
	trustAnchor := cmd.Root().String("mis-trust-anchor")
	misBaseURL := cmd.Root().String("mis-base-url")

	stateFile, svidCert, svidKey := dataDirPaths(cmd)
	state, err := agentstate.Load(stateFile)
	if err != nil {
		return fmt.Errorf("load state (%s): %w", stateFile, err)
	}
	if state.SPIFFEID == "" {
		return fmt.Errorf("state file %s has no spiffe_id; enroll first", stateFile)
	}

	var devKey crypto.PrivateKey
	if cmd.Bool("same-key") {
		signer, err := keygen.LoadSigner(svidKey)
		if err != nil {
			return fmt.Errorf("load same-key signer (%s): %w", svidKey, err)
		}
		devKey = signer
	} else {
		alg := keygen.Algorithm(cmd.String("svid-key-alg"))
		k, err := keygen.Generate(alg)
		if err != nil {
			return err
		}
		devKey = k
	}

	var client *http.Client
	bearerToken := ""
	switch authMode := cmd.String("auth"); authMode {
	case "", "mtls":
		client, err = httpclient.NewMTLS(trustAnchor, svidCert, svidKey)
		if err != nil {
			return fmt.Errorf("load mTLS client from current SVID: %w", err)
		}
	case "bearer":
		client, err = httpclient.New(trustAnchor)
		if err != nil {
			return fmt.Errorf("load HTTPS client for bearer renewal: %w", err)
		}
		signer, err := keygen.LoadSigner(svidKey)
		if err != nil {
			return fmt.Errorf("load current SVID key for bearer exchange: %w", err)
		}
		exchange, err := jwtsvid.Exchange(ctx, jwtsvid.Request{
			Client:     client,
			MISBaseURL: misBaseURL,
			SPIFFEID:   state.SPIFFEID,
			Audiences:  []string{renew.RenewalURL(misBaseURL, state.SPIFFEID)},
			TTL:        5 * time.Minute,
			Mode:       jwtsvid.ModeClientAssertion,
			SigningKey: signer,
		})
		if err != nil {
			return fmt.Errorf("exchange renewal bearer jwt: %w", err)
		}
		if exchange.HTTPStatus < 200 || exchange.HTTPStatus >= 300 {
			return fmt.Errorf("renewal bearer exchange %d: %s", exchange.HTTPStatus, exchange.Problem.Title)
		}
		bearerToken = exchange.Response.JWT
	default:
		return fmt.Errorf("unsupported renewal auth mode %q (want 'mtls' or 'bearer')", authMode)
	}

	res, err := renew.Renew(ctx, renew.Request{
		Client:        client,
		MISBaseURL:    misBaseURL,
		SPIFFEID:      state.SPIFFEID,
		DevicePrivKey: devKey,
		BearerToken:   bearerToken,
	})
	if err != nil {
		return err
	}
	if res.HTTPStatus < 200 || res.HTTPStatus >= 300 {
		return fmt.Errorf("renewal %d: %s", res.HTTPStatus, res.Problem.Title)
	}
	return persistEnrollmentResult(cmd, res.HTTPStatus, res.Response.SVID.CertificateChainPEM, devKey)
}

func persistEnrollmentResult(cmd *cli.Command, statusCode int, chainPEM []string, key crypto.PrivateKey) error {
	// Do all pure-compute/parse work first so any failure aborts before we
	// touch disk. Once MIS has issued the SVID we want either all three
	// artefacts (chain, key, state) to land or none of them - a partial write
	// leaves the device with credentials it cannot locate on next boot.
	if len(chainPEM) == 0 {
		return fmt.Errorf("enrollment response carried no certificate chain")
	}
	chainPEMBytes := []byte(joinPEM(chainPEM))
	leafPEM := chainPEM[0]
	block, _ := pem.Decode([]byte(leafPEM))
	if block == nil {
		return fmt.Errorf("malformed leaf PEM")
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	spiffeID := ""
	if len(leaf.URIs) > 0 {
		spiffeID = leaf.URIs[0].String()
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	state := agentstate.State{
		SPIFFEID:     spiffeID,
		Serial:       leaf.SerialNumber.Text(10),
		ExpiresAt:    leaf.NotAfter.UTC(),
		LastEnrolled: time.Now().UTC(),
	}

	stateOut, chainOut, keyOut := dataDirPaths(cmd)
	if err := writePEMFile(chainOut, chainPEMBytes); err != nil {
		return err
	}
	if err := writePEMFile(keyOut, keyPEM); err != nil {
		return err
	}
	if err := agentstate.Save(stateOut, state); err != nil {
		return err
	}

	fmt.Printf("Enrolled: %s (status %d, expires %s)\n", spiffeID, statusCode, leaf.NotAfter.UTC().Format(time.RFC3339))
	fmt.Printf("Chain written to %s\nKey written to %s\nState written to %s\n", chainOut, keyOut, stateOut)
	return nil
}

func loadFactoryChainPEM(path string) ([]*x509.Certificate, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []*x509.Certificate
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no CERTIFICATE blocks in %s", path)
	}
	return out, nil
}

func loadFactoryKeyPEM(path string) (crypto.Signer, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		s, ok := k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not a crypto.Signer")
		}
		return s, nil
	default:
		return nil, fmt.Errorf("unsupported PEM type %q", block.Type)
	}
}

func writePEMFile(path string, data []byte) error {
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func joinPEM(chain []string) string {
	var out string
	for _, p := range chain {
		out += p
	}
	return out
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

const exitRevocationsRevoked = 10

func checkRevocations(ctx context.Context, cmd *cli.Command) error {
	client, err := httpclient.New(cmd.Root().String("mis-trust-anchor"))
	if err != nil {
		return err
	}
	doc, err := discovery.Fetch(ctx, client, cmd.Root().String("mis-base-url"))
	if err != nil {
		return err
	}
	url := strings.TrimRight(doc.MargoIdentityServiceBaseURI, "/") + "/api/v1/revocations"

	res, err := revocations.Fetch(ctx, client, url,
		cmd.String("if-none-match"), cmd.String("if-modified-since"))
	if err != nil {
		return fmt.Errorf("fetch revocations: %w", err)
	}
	if res.HTTPStatus >= 400 {
		return fmt.Errorf("revocations returned %d: %s", res.HTTPStatus, res.Problem.Title)
	}
	if res.NotModified {
		fmt.Println("304 Not Modified (cached copy is still valid)")
		return nil
	}

	_, svidCert, _ := dataDirPaths(cmd)
	certPEM, err := os.ReadFile(svidCert)
	if err != nil {
		return fmt.Errorf("read svid-cert: %w", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("%s does not contain a CERTIFICATE PEM block", svidCert)
	}
	fp := sha256.Sum256(block.Bytes)
	fpHex := hex.EncodeToString(fp[:])
	spiffeID := ""
	if leaf, err := x509.ParseCertificate(block.Bytes); err == nil && len(leaf.URIs) > 0 {
		spiffeID = leaf.URIs[0].String()
	}

	fmt.Printf("ETag: %s\n", res.ETag)
	fmt.Printf("Last-Modified: %s\n", res.LastModified)
	fmt.Printf("Entries: %d\n", len(res.List.RevokedSVIDs))
	fmt.Printf("Current SVID fingerprint: %s\n", fpHex)
	fmt.Printf("Current SPIFFE ID: %s\n", spiffeID)

	if res.ContainsFingerprint(fpHex) {
		fmt.Println("STATUS: REVOKED")
		os.Exit(exitRevocationsRevoked)
	}
	fmt.Println("STATUS: CLEAN")
	return nil
}
