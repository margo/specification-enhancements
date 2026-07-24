package identitymanager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/margo/miaf-poc/pkg/device-agent/clock"
	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/cache"
	"github.com/margo/miaf-poc/pkg/device-agent/agentstate"
)

// Config bundles every dependency the manager needs.
type Config struct {
	Clock          clock.Clock
	StateFile      string
	SVIDCertFile   string
	SVIDKeyFile    string
	MISBaseURL     string
	MISTrustAnchor string
	RenewAuth      string

	BundleMapURL   string // resolved from discovery at startup
	RevocationsURL string // resolved from discovery at startup

	JWTCache *cache.Cache

	Logger *slog.Logger
}

// Manager owns the only goroutine permitted to mutate device-agent state.
type Manager struct {
	cfg     Config
	cmds    chan Cmd
	state   agentstate.State
	revoked bool
	stopped bool

	once sync.Once
}

// NewManager creates a Manager. Call Run to start it.
func NewManager(cfg Config) *Manager {
	return &Manager{
		cfg:  cfg,
		cmds: make(chan Cmd, 16),
	}
}

// Submit enqueues c. Returns false if the manager has stopped or the context
// is cancelled before the send.
func (m *Manager) Submit(c Cmd) bool {
	if m.stopped {
		return false
	}
	select {
	case m.cmds <- c:
		return true
	case <-c.Ctx.Done():
		return false
	}
}

// Run starts the manager goroutine once. Returns a channel that is closed when
// the goroutine exits.
func (m *Manager) Run(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})
	m.once.Do(func() {
		go m.loop(ctx, done)
	})
	return done
}

func (m *Manager) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	st, err := agentstate.Load(m.cfg.StateFile)
	if err != nil {
		m.cfg.Logger.Error("load state at start", "err", err)
		return
	}
	m.state = st
	m.revoked = !st.RevokedAt.IsZero()

	for {
		select {
		case <-ctx.Done():
			m.cfg.Logger.Info("manager: context cancelled, draining")
			return
		case cmd := <-m.cmds:
			m.dispatch(cmd)
			if cmd.Stop != nil {
				close(cmd.Stop.Reply)
				m.stopped = true
				return
			}
		}
	}
}

func (m *Manager) dispatch(cmd Cmd) {
	switch {
	case cmd.Renew != nil:
		m.handleRenew(cmd.Ctx, cmd.Renew)
	case cmd.Refresh != nil:
		m.handleRefreshBundle(cmd.Ctx, cmd.Refresh)
	case cmd.Check != nil:
		m.handleCheckRevocations(cmd.Ctx, cmd.Check)
	case cmd.JWT != nil:
		m.handleExchangeJWT(cmd.Ctx, cmd.JWT)
	case cmd.Status != nil:
		m.handleStatus(cmd.Ctx, cmd.Status)
	case cmd.Stop != nil:
		// no-op; loop handles channel close
	default:
		m.cfg.Logger.Warn("manager: empty Cmd dropped")
	}
}

// persist commits m.state to disk atomically.
func (m *Manager) persist() error {
	if err := agentstate.Save(m.cfg.StateFile, m.state); err != nil {
		return fmt.Errorf("persist state: %w", err)
	}
	return nil
}

// writeFile writes data to path with 0600 permissions, creating parent dirs.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(dirOf(path), 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", path, err)
	}
	return os.WriteFile(path, data, 0o600)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
