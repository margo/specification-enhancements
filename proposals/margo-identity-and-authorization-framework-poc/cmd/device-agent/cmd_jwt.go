// JWT SVID commands: jwt-exchange and verify-jwt.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/spiffe/go-spiffe/v2/spiffeid"

	"github.com/margo/miaf-poc/pkg/device-agent/bundle"
	"github.com/margo/miaf-poc/pkg/device-agent/discovery"
	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
	jwtverify "github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/verify"
	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

func jwtExchange(ctx context.Context, cmd *cli.Command) error {
	trustAnchor := cmd.Root().String("mis-trust-anchor")
	misBaseURL := cmd.Root().String("mis-base-url")

	stateFile, svidCert, svidKeyPath := dataDirPaths(cmd)
	state, err := agentstate.Load(stateFile)
	if err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	if state.SPIFFEID == "" {
		return fmt.Errorf("state has no spiffe_id; enroll first")
	}

	svidKey, err := keygen.LoadSigner(svidKeyPath)
	if err != nil {
		return fmt.Errorf("load svid-key: %w", err)
	}

	var (
		client *http.Client
		mode   = jwtsvid.ModeMTLS
	)
	switch cmd.String("auth") {
	case "mtls":
		client, err = httpclient.NewMTLS(trustAnchor, svidCert, svidKeyPath)
	case "client-assertion":
		client, err = httpclient.New(trustAnchor)
		mode = jwtsvid.ModeClientAssertion
	default:
		return fmt.Errorf("--auth must be 'mtls' or 'client-assertion'")
	}
	if err != nil {
		return err
	}

	res, err := jwtsvid.Exchange(ctx, jwtsvid.Request{
		Client:     client,
		MISBaseURL: misBaseURL,
		SPIFFEID:   state.SPIFFEID,
		Audiences:  cmd.StringSlice("audience"),
		TTL:        cmd.Duration("ttl"),
		Mode:       mode,
		SigningKey:  svidKey,
	})
	if err != nil {
		return err
	}
	if res.HTTPStatus < 200 || res.HTTPStatus >= 300 {
		return fmt.Errorf("jwt-svid %d: %s", res.HTTPStatus, res.Problem.Title)
	}

	fmt.Printf("Issued JWT SVID for %s (expires %s)\n", state.SPIFFEID, res.Response.ExpiresAt)
	if out := cmd.String("out"); out != "" {
		if err := os.WriteFile(out, []byte(res.Response.JWT), 0o600); err != nil {
			return fmt.Errorf("write --out: %w", err)
		}
		fmt.Printf("Token written to %s\n", out)
	} else {
		fmt.Println(res.Response.JWT)
	}
	return nil
}

func verifyJWT(ctx context.Context, cmd *cli.Command) error {
	compact, err := resolveToken(cmd.String("token"), cmd.String("token-file"))
	if err != nil {
		return fmt.Errorf("resolve token: %w", err)
	}

	client, err := httpclient.New(cmd.Root().String("mis-trust-anchor"))
	if err != nil {
		return err
	}
	doc, err := discovery.Fetch(ctx, client, cmd.Root().String("mis-base-url"))
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	res, err := bundle.Fetch(ctx, client, doc.TrustBundleURI, "")
	if err != nil {
		return fmt.Errorf("bundle fetch: %w", err)
	}
	td, err := spiffeid.TrustDomainFromString(doc.TrustDomain)
	if err != nil {
		return fmt.Errorf("trust domain: %w", err)
	}
	src, err := jwtverify.SourceFromBundleMap(res.Body, td)
	if err != nil {
		return fmt.Errorf("bundle map: %w", err)
	}
	svid, err := jwtverify.ParseAndValidate(ctx, src, cmd.StringSlice("audience"), compact)
	if err != nil {
		return fmt.Errorf("invalid: %w", err)
	}

	fmt.Printf("VALID\n")
	fmt.Printf("  sub:     %s\n", svid.ID)
	fmt.Printf("  aud:     %v\n", svid.Audience)
	fmt.Printf("  expiry:  %s\n", svid.Expiry.UTC().Format(time.RFC3339))
	return nil
}
