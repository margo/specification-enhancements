// Command mis runs the Margo Identity Service HTTP server.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "mis",
		Usage: "Margo Identity Service",
		Flags: []cli.Flag{
			// --- Directories ---
			&cli.StringFlag{Name: "data-dir", Value: "/var/lib/mis", Sources: cli.EnvVars("MIS_DATA_DIR"),
				Usage:    "base directory for MIS state files (derives --db-file default)",
				Category: "Directories"},
			&cli.StringFlag{Name: "etc-dir", Value: "/etc/mis", Sources: cli.EnvVars("MIS_ETC_DIR"),
				Usage:    "base directory for MIS config and key files (derives file flag defaults)",
				Category: "Directories"},

			// --- Network / TLS ---
			&cli.StringFlag{Name: "trust-domain", Value: "margo.example.com", Sources: cli.EnvVars("MIS_TRUST_DOMAIN"),
				Category: "Network / TLS"},
			&cli.StringFlag{Name: "bind-address", Value: "0.0.0.0:8443", Sources: cli.EnvVars("MIS_BIND_ADDRESS"),
				Category: "Network / TLS"},
			&cli.StringFlag{Name: "advertise-url", Value: "https://localhost:8443", Sources: cli.EnvVars("MIS_ADVERTISE_URL"),
				Category: "Network / TLS"},
			&cli.StringFlag{Name: "tls-cert-file", Sources: cli.EnvVars("MIS_TLS_CERT_FILE"),
				Category: "Network / TLS"},
			&cli.StringFlag{Name: "tls-key-file", Sources: cli.EnvVars("MIS_TLS_KEY_FILE"),
				Category: "Network / TLS"},

			// --- CA / trust ---
			&cli.StringFlag{Name: "ca-trust-anchor-file", Sources: cli.EnvVars("MIS_CA_TRUST_ANCHOR_FILE"),
				Usage:    "PEM file of X.509 root CA certificates trusted as SPIFFE Bundle Map anchors; used by both issuer backends",
				Category: "CA / Trust"},

			// --- Issuer backend ---
			&cli.StringFlag{Name: "issuer-backend", Value: "intermediate", Sources: cli.EnvVars("MIS_ISSUER_BACKEND"),
				Usage:    "Issuer backend: 'intermediate' (MIS signs in-process) or 'stepca-ra' (MIS delegates to Step CA via admin API)",
				Category: "Issuer Backend"},
			&cli.StringFlag{Name: "ca-intermediate-cert-file", Sources: cli.EnvVars("MIS_CA_INTERMEDIATE_CERT_FILE"),
				Usage:    "PEM certificate for MIS's Intermediate CA (SPIFFE ID spiffe://<td>/)",
				Category: "Issuer Backend"},
			&cli.StringFlag{Name: "ca-intermediate-key-file", Sources: cli.EnvVars("MIS_CA_INTERMEDIATE_KEY_FILE"),
				Usage:    "PEM private key matching --ca-intermediate-cert-file",
				Category: "Issuer Backend"},
			&cli.StringFlag{Name: "stepca-url", Value: "https://step-ca:9000", Sources: cli.EnvVars("MIS_STEPCA_URL"),
				Usage:    "Base URL of the Step CA instance used by the stepca-ra issuer backend",
				Category: "Issuer Backend"},
			&cli.StringFlag{Name: "stepca-provisioner-name", Value: "margo-mis", Sources: cli.EnvVars("MIS_STEPCA_PROVISIONER_NAME"),
				Usage:    "Name of the Step CA JWK provisioner MIS authenticates as",
				Category: "Issuer Backend"},
			&cli.StringFlag{Name: "stepca-provisioner-file", Sources: cli.EnvVars("MIS_STEPCA_PROVISIONER_FILE"),
				Usage:    "Path to the exported Step CA provisioner JSON (public JWK + encrypted private JWK)",
				Category: "Issuer Backend"},
			&cli.StringFlag{Name: "stepca-password-file", Sources: cli.EnvVars("MIS_STEPCA_PASSWORD_FILE"),
				Usage:    "Path to the Step CA provisioner password file",
				Category: "Issuer Backend"},

			// --- Bootstrap methods ---
			&cli.StringSliceFlag{Name: "factory-ca-file", Sources: cli.EnvVars("MIS_FACTORY_CA_FILE"),
				Usage:    "factory-cert-mtls / factory-cert-jwt: PEM file of factory root CAs trusted for device bootstrap (repeat for multiple CAs)",
				Category: "Bootstrap Methods"},
			&cli.DurationFlag{Name: "bootstrap-jwt-clock-skew", Value: 30 * time.Second, Sources: cli.EnvVars("MIS_BOOTSTRAP_JWT_CLOCK_SKEW"),
				Usage:    "factory-cert-jwt: Allowed clock skew for exp/nbf claims",
				Category: "Bootstrap Methods"},
			&cli.DurationFlag{Name: "enrollment-token-retry-window", Value: 5 * time.Minute, Sources: cli.EnvVars("MIS_ENROLLMENT_TOKEN_RETRY_WINDOW"),
				Usage:    "enrollment-token: Idempotent-retry recognition window for enrollment tokens (max: 15m)",
				Category: "Bootstrap Methods"},

			// --- SVID issuance / renewal policy ---
			&cli.DurationFlag{Name: "svid-ttl", Value: 90 * 24 * time.Hour, Sources: cli.EnvVars("MIS_SVID_TTL"),
				Usage:    "Lifetime of issued device X.509 SVIDs",
				Category: "SVID Issuance / Renewal Policy"},
			&cli.BoolFlag{Name: "allow-new-enrollment", Value: true, Sources: cli.EnvVars("MIS_ALLOW_NEW_ENROLLMENT"),
				Usage:    "Allow enrollment of new devices (when false, only existing identities may re-enroll)",
				Category: "SVID Issuance / Renewal Policy"},
			&cli.BoolFlag{Name: "allow-key-rotation", Value: true, Sources: cli.EnvVars("MIS_ALLOW_KEY_ROTATION"),
				Usage:    "Allow a device to rotate its key pair on re-enrollment",
				Category: "SVID Issuance / Renewal Policy"},
			&cli.IntFlag{Name: "renewal-rate-limit", Value: 5, Sources: cli.EnvVars("MIS_RENEWAL_RATE_LIMIT"),
				Usage:    "Maximum successful renewals per identity per --renewal-rate-window",
				Category: "SVID Issuance / Renewal Policy"},
			&cli.DurationFlag{Name: "renewal-rate-window", Value: 24 * time.Hour, Sources: cli.EnvVars("MIS_RENEWAL_RATE_WINDOW"),
				Usage:    "Sliding window for per-identity renewal rate limiting",
				Category: "SVID Issuance / Renewal Policy"},

			// --- JWT SVID exchange ---
			&cli.StringFlag{Name: "jwtsvid-key-file", Sources: cli.EnvVars("MIS_JWTSVID_KEY_FILE"),
				Usage:    "PEM private key (PKCS#8) used to sign JWT SVIDs",
				Category: "JWT SVID Exchange"},
			&cli.DurationFlag{Name: "jwtsvid-default-ttl", Value: 5 * time.Minute, Sources: cli.EnvVars("MIS_JWTSVID_DEFAULT_TTL"),
				Usage:    "Default lifetime of the issued JWT SVID when the request omits TTL",
				Category: "JWT SVID Exchange"},
			&cli.DurationFlag{Name: "jwtsvid-max-ttl", Value: time.Hour, Sources: cli.EnvVars("MIS_JWTSVID_MAX_TTL"),
				Usage:    "Upper bound on issued JWT SVID lifetime",
				Category: "JWT SVID Exchange"},
			&cli.DurationFlag{Name: "jwtsvid-clock-skew", Value: 30 * time.Second, Sources: cli.EnvVars("MIS_JWTSVID_CLOCK_SKEW"),
				Usage:    "Allowed clock skew for exp/nbf claims",
				Category: "JWT SVID Exchange"},

			// --- Revocation ---
			&cli.DurationFlag{Name: "revocation-prune-interval", Value: time.Hour, Sources: cli.EnvVars("MIS_REVOCATION_PRUNE_INTERVAL"),
				Usage:    "Interval at which MIS removes naturally-expired entries from its revocation list",
				Category: "Revocation"},

			// --- Storage ---
			&cli.StringFlag{Name: "db-file", Sources: cli.EnvVars("MIS_DB_FILE"),
				Usage:    "SQLite path for unified MIS state (identities, enrollment tokens, jwt replay, replacement tickets)",
				Category: "Storage"},

			// --- Admin API ---
			&cli.StringFlag{Name: "admin-bind-address", Value: "127.0.0.1:8445", Sources: cli.EnvVars("MIS_ADMIN_BIND_ADDRESS"),
				Usage:    "Bind address for the NON-NORMATIVE /admin/v1 HTTP API (plaintext)",
				Category: "Admin API"},
			&cli.StringFlag{Name: "admin-url", Value: "http://127.0.0.1:8445", Sources: cli.EnvVars("MIS_ADMIN_URL"),
				Usage:    "URL the generate-token, generate-replacement-ticket, and revoke subcommands target",
				Category: "Admin API"},
			&cli.DurationFlag{Name: "enrollment-token-default-ttl", Value: time.Hour, Sources: cli.EnvVars("MIS_ENROLLMENT_TOKEN_DEFAULT_TTL"),
				Usage:    "Default lifetime for issued enrollment tokens",
				Category: "Admin API"},
		},
		Commands: []*cli.Command{
			{
				Name:  "generate-token",
				Usage: "Generate an enrollment token via MIS's non-normative admin API",
				Flags: []cli.Flag{
					&cli.DurationFlag{Name: "ttl", Usage: "Requested lifetime for issued enrollment token"},
				},
				Action: runGenerateToken,
			},
			{
				Name:  "generate-replacement-ticket",
				Usage: "Generate a single-use replacement ticket via MIS's non-normative admin API",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "spiffe-id", Required: true, Usage: "SPIFFE ID of the device identity to replace"},
					&cli.DurationFlag{Name: "ttl", Value: time.Hour, Usage: "Lifetime of the issued replacement ticket"},
				},
				Action: runGenerateReplacementTicket,
			},
			{
				Name:  "revoke",
				Usage: "Revoke a SPIFFE ID via MIS's non-normative admin API",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "spiffe-id", Required: true, Usage: "SPIFFE ID of the device identity to revoke"},
					&cli.StringFlag{Name: "reason", Usage: "Operator-supplied reason string"},
				},
				Action: runRevoke,
			},
		},
		Action: run,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}
