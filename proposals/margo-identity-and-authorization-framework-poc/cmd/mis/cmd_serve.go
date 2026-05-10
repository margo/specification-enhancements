// MIS server startup and lifecycle management.
package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/enrollmenttoken"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertjwt"
	"github.com/margo/miaf-poc/pkg/mis/bootstrap/factorycertmtls"
	"github.com/margo/miaf-poc/pkg/mis/bundlemap"
	"github.com/margo/miaf-poc/pkg/mis/ca"
	"github.com/margo/miaf-poc/pkg/mis/identity"
	"github.com/margo/miaf-poc/pkg/mis/jwtsigner"
	"github.com/margo/miaf-poc/pkg/mis/ra/stepca"
	"github.com/margo/miaf-poc/pkg/mis/store"
	"github.com/margo/miaf-poc/pkg/mis/transport/admin"
	mishttp "github.com/margo/miaf-poc/pkg/mis/transport/http"
)

type resolvedPaths struct {
	dbFile             string
	tlsCertFile        string
	tlsKeyFile         string
	caTrustAnchorFile  string
	caIntermediateCert string
	caIntermediateKey  string
	jwtsvidKeyFile     string
	stepCAProvFile     string
	stepCAPassFile     string
	factoryCaFiles     []string
}

func resolvePaths(cmd *cli.Command) resolvedPaths {
	dataDir := cmd.String("data-dir")
	etcDir := cmd.String("etc-dir")
	pick := func(flag, rel, base string) string {
		if v := cmd.String(flag); v != "" {
			return v
		}
		return filepath.Join(base, rel)
	}
	factoryFiles := cmd.StringSlice("factory-ca-file")
	if len(factoryFiles) == 0 {
		factoryFiles = []string{filepath.Join(etcDir, "factory/root.crt")}
	}
	return resolvedPaths{
		dbFile:             pick("db-file", "mis.sqlite3", dataDir),
		tlsCertFile:        pick("tls-cert-file", "tls/server.crt", etcDir),
		tlsKeyFile:         pick("tls-key-file", "tls/server.key", etcDir),
		caTrustAnchorFile:  pick("ca-trust-anchor-file", "ca/root.crt", etcDir),
		caIntermediateCert: pick("ca-intermediate-cert-file", "ca/mis-intermediate.crt", etcDir),
		caIntermediateKey:  pick("ca-intermediate-key-file", "ca/mis-intermediate.key", etcDir),
		jwtsvidKeyFile:     pick("jwtsvid-key-file", "jwtsvid/jwtsvid.key", etcDir),
		stepCAProvFile:     pick("stepca-provisioner-file", "stepca/provisioner.json", etcDir),
		stepCAPassFile:     pick("stepca-password-file", "stepca/password", etcDir),
		factoryCaFiles:     factoryFiles,
	}
}

func run(ctx context.Context, cmd *cli.Command) error {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	paths := resolvePaths(cmd)
	trustDomain := cmd.String("trust-domain")
	baseURL := cmd.String("advertise-url")
	baseURLNoSlash := strings.TrimRight(baseURL, "/")

	var issuer identity.Issuer
	backend := cmd.String("issuer-backend")
	switch backend {
	case "intermediate":
		intIssuer, err := ca.NewIntermediateIssuer(ca.Config{
			TrustDomain: trustDomain,
			CertFile:    paths.caIntermediateCert,
			KeyFile:     paths.caIntermediateKey,
		})
		if err != nil {
			return fmt.Errorf("load intermediate CA: %w", err)
		}
		issuer = intIssuer
		logger.Info("MIS issuer backend: intermediate",
			"intermediate_cert", paths.caIntermediateCert,
		)
	case "stepca-ra":
		cred, err := stepca.LoadCredential(stepca.CredentialConfig{
			ProvisionerFile: paths.stepCAProvFile,
			PasswordFile:    paths.stepCAPassFile,
		})
		if err != nil {
			return fmt.Errorf("load stepca credential: %w", err)
		}
		rootPool, err := buildCombinedPool(paths.caTrustAnchorFile)
		if err != nil {
			return fmt.Errorf("load stepca root pool: %w", err)
		}
		raClient, err := stepca.NewClient(stepca.ClientConfig{
			BaseURL:    cmd.String("stepca-url"),
			RootPool:   rootPool,
			Credential: cred,
		})
		if err != nil {
			return fmt.Errorf("build stepca client: %w", err)
		}
		raIssuer, err := stepca.NewIssuer(stepca.IssuerConfig{
			TrustDomain: trustDomain,
			Client:      raClient,
		})
		if err != nil {
			return fmt.Errorf("build stepca issuer: %w", err)
		}
		issuer = raIssuer
		logger.Info("MIS issuer backend: stepca-ra",
			"stepca_url", cmd.String("stepca-url"),
			"provisioner", cred.ProvisionerName,
		)
	default:
		return fmt.Errorf("unknown --issuer-backend %q (want 'intermediate' or 'stepca-ra')", backend)
	}

	jwtSigner, err := jwtsigner.New(jwtsigner.Config{KeyFile: paths.jwtsvidKeyFile})
	if err != nil {
		return fmt.Errorf("load jwt signer: %w", err)
	}

	trustAnchors, err := ca.NewTrustAnchors(ca.TrustAnchorsConfig{
		TrustDomain:    trustDomain,
		RootFile:       paths.caTrustAnchorFile,
		JWTAuthorities: []bundlemap.JWTAuthority{jwtSigner.JWTAuthority()},
	})
	if err != nil {
		return fmt.Errorf("load trust anchors: %w", err)
	}

	st, err := store.Open(paths.dbFile)
	if err != nil {
		return fmt.Errorf("open mis state: %w", err)
	}
	defer st.Close()

	repo := st.Identities()
	replValidator := st.ReplacementTickets()

	policy := identity.StaticPolicy{
		NewLDI:      cmd.Bool("allow-new-enrollment"),
		KeyRotation: cmd.Bool("allow-key-rotation"),
	}

	issuanceLog := st.IssuedSVIDs()
	revocationsQ := st.RevokedSVIDs()
	svc := identity.NewService(repo, issuer, policy, replValidator, issuanceLog, revocationsQ, trustDomain, cmd.Duration("svid-ttl"))

	renewalLimiter := st.RenewalEvents(store.RenewalEventsConfig{
		MaxInWindow: int(cmd.Int("renewal-rate-limit")),
		Window:      cmd.Duration("renewal-rate-window"),
	})
	renewalHandler := mishttp.NewRenewalHandler(mishttp.RenewalConfig{
		Service:         svc,
		RateLimiter:     renewalLimiter,
		BearerVerifier:  mishttp.NewLocalRenewalBearerVerifier(trustAnchors),
		EndpointBaseURL: baseURL,
		Logger:          logger,
	})

	factoryPool, err := buildCombinedPool(paths.factoryCaFiles...)
	if err != nil {
		return err
	}
	// Combined TLS ClientCAs pool: factory root CAs (factory-cert-*) +
	// MIS root CA (so Go's TLS stack includes the SVID client cert on renewal).
	clientCAPool, err := buildCombinedPool(append(paths.factoryCaFiles, paths.caTrustAnchorFile)...)
	if err != nil {
		return fmt.Errorf("build TLS client CA pool: %w", err)
	}
	registry := bootstrap.NewRegistry()
	registry.Register(common.BootstrapMethodFactoryCertMTLS, factorycertmtls.New(factoryPool))

	jwtAudience := baseURLNoSlash + "/api/v1/identities"
	registry.Register(common.BootstrapMethodFactoryCertJWT, factorycertjwt.New(factorycertjwt.Config{
		Roots:     factoryPool,
		Audience:  jwtAudience,
		Replay:    st.JWTReplay(),
		ClockSkew: cmd.Duration("bootstrap-jwt-clock-skew"),
	}))

	registry.Register(common.BootstrapMethodEnrollmentToken, enrollmenttoken.New(enrollmenttoken.Config{
		Store:       st.EnrollmentTokens(),
		RetryWindow: cmd.Duration("enrollment-token-retry-window"),
	}))

	doc := &common.DiscoveryResponseDTO{
		TrustDomain:                 trustDomain,
		TrustBundleURI:              baseURL + "/.well-known/spiffe/bundle.json",
		MargoIdentityServiceBaseURI: baseURL,
		SupportedBootstrapMethods:   registry.Methods(),
		SVIDProfilesSupported: []string{
			common.SVIDProfileURIX509V1,
			common.SVIDProfileURIJWTV1,
		},
	}

	revocationsHandler := mishttp.NewRevocationsHandler(mishttp.RevocationsConfig{
		Queue: revocationsQ,
	})

	jwtSVIDHandler := mishttp.NewJWTSVIDHandler(mishttp.JWTSVIDConfig{
		Service:      svc,
		Signer:       jwtSigner,
		IssuanceLog:  st.IssuedJWTSVIDs(),
		BundleSource: trustAnchors,
		ReplayStore:  st.JWTReplay(),
		RateLimiter:  renewalLimiter,
		IssuerURL:    baseURLNoSlash + "/",
		EndpointBase: baseURL,
		MaxTTL:       cmd.Duration("jwtsvid-max-ttl"),
		DefaultTTL:   cmd.Duration("jwtsvid-default-ttl"),
		ClockSkew:    cmd.Duration("jwtsvid-clock-skew"),
		Logger:       logger,
	})

	srv := mishttp.NewServer(mishttp.Config{
		Logger:             logger,
		DiscoveryHandler:   mishttp.NewDiscoveryHandler(doc),
		BundleMapHandler:   mishttp.NewBundleMapHandler(trustAnchors, 10*time.Minute),
		EnrollmentHandler:  mishttp.NewEnrollmentHandler(mishttp.EnrollmentConfig{Registry: registry, Service: svc, Logger: logger}),
		RenewalHandler:     renewalHandler,
		RevocationsHandler: revocationsHandler,
		JWTSVIDHandler:     jwtSVIDHandler,
		ClientCAPool:       clientCAPool,
	})

	adminSrv := admin.NewServer(admin.Config{
		EnrollmentTokens:   st.EnrollmentTokens(),
		ReplacementTickets: st.ReplacementTickets(),
		Revocations:        svc,
		DefaultTokenTTL:    cmd.Duration("enrollment-token-default-ttl"),
		Logger:             logger,
	})

	logger.Info("starting MIS",
		"bind", cmd.String("bind-address"),
		"admin_bind", cmd.String("admin-bind-address"),
		"trust_domain", trustDomain,
		"advertise_url", baseURL,
		"methods", registry.Methods(),
	)

	go func(interval time.Duration) {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				n, err := revocationsQ.PruneExpired(ctx, time.Now())
				if err != nil {
					logger.Warn("revocation prune failed", "err", err.Error())
					continue
				}
				if n > 0 {
					logger.Info("revocation prune", "removed", n)
				}
			}
		}
	}(cmd.Duration("revocation-prune-interval"))

	errCh := make(chan error, 2)
	go func() {
		errCh <- srv.ListenAndServeTLS(ctx, cmd.String("bind-address"),
			paths.tlsCertFile, paths.tlsKeyFile)
	}()
	go func() {
		errCh <- adminSrv.ListenAndServe(ctx, cmd.String("admin-bind-address"))
	}()

	return <-errCh
}

func buildCombinedPool(files ...string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		if !pool.AppendCertsFromPEM(b) {
			return nil, fmt.Errorf("no PEM certificates in %s", f)
		}
	}
	return pool, nil
}
