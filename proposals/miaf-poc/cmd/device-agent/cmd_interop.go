// SPIFFE Interoperability Demo command: interop-call.
package main

import (
	"context"
	"fmt"

	"github.com/urfave/cli/v3"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"

	"github.com/margo/miaf-poc/pkg/device-agent/bundle"
	"github.com/margo/miaf-poc/pkg/device-agent/discovery"
	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	"github.com/margo/miaf-poc/pkg/device-agent/interop"
)

func interopCall(ctx context.Context, cmd *cli.Command) error {
	tdStr := cmd.String("trust-domain")
	td, err := spiffeid.TrustDomainFromString(tdStr)
	if err != nil {
		return fmt.Errorf("trust-domain: %w", err)
	}

	client, err := httpclient.New(cmd.Root().String("mis-trust-anchor"))
	if err != nil {
		return err
	}
	doc, err := discovery.Fetch(ctx, client, cmd.Root().String("mis-base-url"))
	if err != nil {
		return err
	}
	result, err := bundle.Fetch(ctx, client, doc.TrustBundleURI, "")
	if err != nil {
		return err
	}
	if result.NotModified {
		return fmt.Errorf("bundle map returned 304 with no cached body - first call must fetch")
	}
	authorities, err := bundle.ExtractX509Authorities(result.Body, tdStr)
	if err != nil {
		return err
	}
	bundleSrc := x509bundle.FromX509Authorities(td, authorities)

	_, svidCert, svidKey := dataDirPaths(cmd)
	svid, err := x509svid.Load(svidCert, svidKey)
	if err != nil {
		return fmt.Errorf("load SVID: %w", err)
	}

	resp, err := interop.Dial(ctx, interop.DialInput{
		ServerAddr:   cmd.String("server"),
		TrustDomain:  td,
		ClientSVID:   svid,
		BundleSource: bundleSrc,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Device SPIFFE ID: %s\n", svid.ID)
	fmt.Printf("Workload SPIFFE ID (from peer cert): %s\n", resp.PeerSPIFFEID)
	fmt.Printf("Workload's self-reported SPIFFE ID: %s\n", resp.ServerSPIFFEID)
	return nil
}
