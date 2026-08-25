package daemon

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/clock"
	"github.com/margo/miaf-poc/pkg/device-agent/discovery"
	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
	jwtcache "github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/cache"
	"github.com/margo/miaf-poc/pkg/device-agent/scheduler"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

// Config holds all daemon startup parameters.
type Config struct {
	StateFile      string
	SVIDCertFile   string
	SVIDKeyFile    string
	MISBaseURL     string
	MISTrustAnchor string
	RenewAuth      string

	SocketPath string // default: /run/device-agent/daemon.sock

	BundleEvery      time.Duration // default 1h
	RevocationsEvery time.Duration // default 5m
	RenewalRatio     float64       // default 0.5
	JWTCacheTTL      time.Duration // default 5m

	Logger *slog.Logger
	Clock  clock.Clock
}

// Run starts the daemon and blocks until SIGTERM/SIGINT or ctx is cancelled.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Clock == nil {
		cfg.Clock = clock.Real{}
	}
	if cfg.SocketPath == "" {
		cfg.SocketPath = "/run/device-agent/daemon.sock"
	}
	if cfg.JWTCacheTTL == 0 {
		cfg.JWTCacheTTL = 5 * time.Minute
	}

	// Load state to get leaf cert times for the scheduler.
	st, err := agentstate.Load(cfg.StateFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("no identity state at %s: run \"device-agent enroll\" first", cfg.StateFile)
		}
		return fmt.Errorf("load state: %w", err)
	}
	leaf, err := readLeaf(cfg.SVIDCertFile)
	if err != nil {
		return fmt.Errorf("read leaf cert: %w", err)
	}

	// Resolve discovery document once at startup.
	cli, err := httpclient.New(cfg.MISTrustAnchor)
	if err != nil {
		return err
	}
	doc, err := discovery.Fetch(ctx, cli, cfg.MISBaseURL)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}

	jwtCache := jwtcache.New(cfg.JWTCacheTTL)
	mgr := identitymanager.NewManager(identitymanager.Config{
		Clock:          cfg.Clock,
		StateFile:      cfg.StateFile,
		SVIDCertFile:   cfg.SVIDCertFile,
		SVIDKeyFile:    cfg.SVIDKeyFile,
		MISBaseURL:     cfg.MISBaseURL,
		MISTrustAnchor: cfg.MISTrustAnchor,
		RenewAuth:      cfg.RenewAuth,
		BundleMapURL:   doc.TrustBundleURI,
		RevocationsURL: strings.TrimRight(doc.MargoIdentityServiceBaseURI, "/") + "/api/v1/revocations",
		JWTCache:       jwtCache,
		Logger:         cfg.Logger,
	})

	sch := scheduler.New(scheduler.Config{
		Clock:            cfg.Clock,
		Manager:          mgr,
		JWTCache:         jwtCache,
		BundleEvery:      cfg.BundleEvery,
		RevocationsEvery: cfg.RevocationsEvery,
		JWTEvictEvery:    30 * time.Second,
		RenewalRatio:     cfg.RenewalRatio,
		LeafNotBefore:    leaf.NotBefore,
		LeafNotAfter:     leaf.NotAfter,
		Logger:           cfg.Logger,
	})

	sock := newSocketServer(cfg.SocketPath, mgr, cfg.Logger)

	signalCtx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mgrDone := mgr.Run(signalCtx)
	go sch.Run(signalCtx)
	if err := sock.Start(signalCtx); err != nil {
		return fmt.Errorf("start socket: %w", err)
	}

	cfg.Logger.Info("daemon: running",
		"spiffe_id", st.SPIFFEID, "socket", cfg.SocketPath)
	<-signalCtx.Done()
	cfg.Logger.Info("daemon: signal received, draining")

	stopReply := make(chan struct{}, 1)
	mgr.Submit(identitymanager.Cmd{
		Ctx:  context.Background(),
		Stop: &identitymanager.StopCmd{Reply: stopReply},
	})
	// Wait for Stop to be processed OR for the manager to have already exited
	// via ctx.Done() - whichever fires first prevents a deadlock.
	select {
	case <-stopReply:
	case <-mgrDone:
	}
	<-mgrDone
	sock.Close()
	cfg.Logger.Info("daemon: stopped")
	return nil
}

func readLeaf(certPath string) (*x509.Certificate, error) {
	raw, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", certPath)
	}
	return x509.ParseCertificate(block.Bytes)
}
