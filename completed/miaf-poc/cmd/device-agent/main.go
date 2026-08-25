// Command device-agent is the device agent for an Edge Compute Device.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "device-agent",
		Usage: "Margo Device Agent for an Edge Compute Device",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "mis-base-url", Value: "https://localhost:8443"},
			&cli.StringFlag{Name: "mis-trust-anchor", Value: "",
				Usage: "PEM file to pin as the MIS TLS trust anchor; empty = system trust store"},
			&cli.StringFlag{Name: "data-dir", Value: "/var/lib/device-agent", Usage: "directory for state and SVID files (overridden by --state-file/--svid-cert/--svid-key)"},
			&cli.StringFlag{Name: "state-file", Value: "", Usage: "path to persisted device enrollment state (default: <data-dir>/state.json)"},
			&cli.StringFlag{Name: "svid-cert", Value: "", Usage: "PEM file with the current SVID certificate chain (default: <data-dir>/svid.pem)"},
			&cli.StringFlag{Name: "svid-key", Value: "", Usage: "PEM file with the current SVID private key (default: <data-dir>/svid.key)"},
		},
		Commands: []*cli.Command{
			{Name: "get-discovery", Category: "1) MIS Discovery & Trust", Usage: "Fetch and print the MIAF discovery document", Action: getDiscovery},
			{Name: "get-bundle", Category: "1) MIS Discovery & Trust", Usage: "Fetch and print the SPIFFE Bundle Map", Flags: []cli.Flag{
				&cli.StringFlag{Name: "if-modified-since", Usage: "Optional HTTP-date for a conditional GET"},
			}, Action: getBundle},
			{
				Name:     "enroll",
				Category: "2) Device Identity Lifecycle",
				Usage:    "Enroll via a MIAF bootstrap method",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "svid-key-alg", Value: "ES256", Usage: "ES256 or PS256"},
					&cli.StringFlag{Name: "replacement-ticket", Value: "", Usage: "optional operator-ticket for replacement"},
				},
				Commands: []*cli.Command{
					{
						Name:  "factory-cert-mtls",
						Usage: "Enroll using a manufacturer X.509 client certificate over mTLS",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "factory-cert", Required: true, Usage: "PEM factory device certificate"},
							&cli.StringFlag{Name: "factory-key", Required: true, Usage: "PEM factory device private key"},
						},
						Action: enrollFactoryCertMTLS,
					},
					{
						Name:  "factory-cert-jwt",
						Usage: "Enroll using a Bootstrap Assertion JWT signed with the factory key (no mTLS)",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "factory-cert", Required: true, Usage: "PEM factory device certificate"},
							&cli.StringFlag{Name: "factory-key", Required: true, Usage: "PEM factory device private key"},
						},
						Action: enrollFactoryCertJWT,
					},
					{
						Name:  "enrollment-token",
						Usage: "Enroll using an operator-issued single-use enrollment token (no factory cert required)",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "token", Usage: "Enrollment token in wire format (margo-et-v1.<base64url>); use --token-file for a less leaky alternative"},
							&cli.StringFlag{Name: "token-file", Usage: "Path to a file whose trimmed contents are the enrollment token"},
						},
						Action: enrollEnrollmentToken,
					},
				},
			},
			{
				Name:     "renew",
				Category: "2) Device Identity Lifecycle",
				Usage:    "Renew the current SVID using either mTLS or a JWT-SVID bearer token",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "svid-key-alg", Value: "ES256", Usage: "ES256 or PS256"},
					&cli.StringFlag{Name: "auth", Value: "mtls", Usage: "'mtls' (default) or 'bearer'"},
					&cli.BoolFlag{Name: "same-key", Value: false, Usage: "re-use the current private key (default: generate a new one)"},
				},
				Action: renewSVID,
			},
			{
				Name:     "jwt-exchange",
				Category: "3) JWT SVID Operations",
				Usage:    "Exchange the current X.509 SVID for a short-lived JWT SVID",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: "audience", Required: true,
						Usage: "Repeatable: audience identifier to put in the JWT SVID aud claim"},
					&cli.DurationFlag{Name: "ttl", Value: 5 * time.Minute,
						Usage: "Requested JWT SVID lifetime (capped by MIS policy)"},
					&cli.StringFlag{Name: "auth", Value: "client-assertion",
						Usage: "'client-assertion' (default) or 'mtls'"},
					&cli.StringFlag{Name: "out", Value: "",
						Usage: "Optional path to write the issued JWT SVID; default: stdout"},
				},
				Action: jwtExchange,
			},
			{
				Name:     "verify-jwt",
				Category: "3) JWT SVID Operations",
				Usage:    "Verify a JWT SVID against the MIS Trust Bundle",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "token", Usage: "Compact JWT SVID"},
					&cli.StringFlag{Name: "token-file", Usage: "Path to a file whose contents are the compact JWT SVID"},
					&cli.StringSliceFlag{Name: "audience", Required: true,
						Usage: "Repeatable: expected audience to validate against"},
				},
				Action: verifyJWT,
			},
			{
				Name:     "check-revocations",
				Category: "2) Device Identity Lifecycle",
				Usage:    "Fetch the MIAF revocation list and report whether the current SVID is listed",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "if-none-match", Value: "", Usage: "Optional ETag for a conditional GET"},
					&cli.StringFlag{Name: "if-modified-since", Value: "", Usage: "Optional HTTP-date for a conditional GET"},
				},
				Action: checkRevocations,
			},
			{
				Name:     "daemon",
				Category: "4) Device Agent Daemon",
				Usage:    "Run as a long-lived device agent (automatic renewal, bundle refresh, revocation checks, on-demand JWT exchange)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "socket", Value: "/run/device-agent/daemon.sock",
						Usage: "Unix domain socket path; the ctl subcommand connects here to send commands to the running daemon"},
					&cli.DurationFlag{Name: "bundle-every", Value: time.Hour,
						Usage: "interval between periodic Trust Bundle refreshes from the MIS"},
					&cli.DurationFlag{Name: "revocations-every", Value: 5 * time.Minute,
						Usage: "interval between periodic revocation list polls from the MIS"},
					&cli.StringFlag{Name: "renew-auth", Value: "mtls",
						Usage: "renewal authentication mode: 'mtls' (default) or 'bearer'"},
					&cli.Float64Flag{Name: "renewal-ratio", Value: 0.5,
						Usage: "fraction of X.509 SVID lifetime elapsed before automatic renewal fires (e.g. 0.5 = halfway through)"},
					&cli.DurationFlag{Name: "jwt-cache-ttl", Value: 5 * time.Minute,
						Usage: "minimum remaining lifetime a cached JWT SVID must have before it is served by ctl exchange-jwt"},
				},
				Action: runDaemon,
			},
			{
				Name:     "ctl",
				Category: "4) Device Agent Daemon",
				Usage:    "Send a command to a running device-agent daemon",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "socket", Value: "/run/device-agent/daemon.sock",
						Usage: "Unix domain socket path of the daemon to connect to"},
				},
				Commands: []*cli.Command{
					{Name: "status", Usage: "Print the daemon's current identity state", Action: ctlStatus},
					{
						Name:  "exchange-jwt",
						Usage: "Request a JWT SVID from the daemon (served from cache or freshly exchanged with the MIS)",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{Name: "audience", Required: true,
								Usage: "Repeatable: audience identifier to include in the JWT SVID aud claim"},
							&cli.DurationFlag{Name: "ttl", Value: 5 * time.Minute,
								Usage: "Requested JWT SVID lifetime (capped by MIS policy)"},
						},
						Action: ctlExchangeJWT,
					},
					{Name: "force-renew", Usage: "Trigger an immediate SVID renewal", Action: ctlForceRenew},
					{Name: "force-bundle-refresh", Usage: "Trigger an immediate Trust Bundle refresh", Action: ctlForceBundle},
					{Name: "force-revocations-check", Usage: "Trigger an immediate revocation list poll", Action: ctlForceRevocations},
				},
			},
			{
				Name:     "interop-call",
				Category: "5) SPIFFE Interoperability Demo",
				Usage:    "Dial a SPIRE-attested workload via mTLS using the current SVID issued by MIS",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "server", Required: true, Usage: "host:port of the spire-workload echo server"},
					&cli.StringFlag{Name: "trust-domain", Value: "margo.example.com", Usage: "Trust Domain expected on the peer SPIFFE ID"},
				},
				Action: interopCall,
			},
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func resolveToken(inline, fromFile string) (string, error) {
	if inline != "" && fromFile != "" {
		return "", fmt.Errorf("--token and --token-file are mutually exclusive")
	}
	if inline != "" {
		return inline, nil
	}
	if fromFile == "" {
		return "", fmt.Errorf("one of --token or --token-file is required")
	}
	raw, err := os.ReadFile(fromFile)
	if err != nil {
		return "", fmt.Errorf("read --token-file: %w", err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func jsonPrint(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// dataDirPaths resolves the three data file paths. Individual flags override
// the data-dir-derived defaults, allowing fine-grained control when needed.
func dataDirPaths(cmd *cli.Command) (stateFile, svidCert, svidKey string) {
	dir := cmd.Root().String("data-dir")
	stateFile = cmd.Root().String("state-file")
	svidCert = cmd.Root().String("svid-cert")
	svidKey = cmd.Root().String("svid-key")
	if stateFile == "" {
		stateFile = filepath.Join(dir, "state.json")
	}
	if svidCert == "" {
		svidCert = filepath.Join(dir, "svid.pem")
	}
	if svidKey == "" {
		svidKey = filepath.Join(dir, "svid.key")
	}
	return
}
