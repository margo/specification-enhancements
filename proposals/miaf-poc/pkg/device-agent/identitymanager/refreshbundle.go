package identitymanager

import (
	"context"

	"github.com/margo/miaf-poc/pkg/device-agent/bundle"
	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
)

func (m *Manager) handleRefreshBundle(ctx context.Context, c *RefreshBundleCmd) {
	defer close(c.Reply)

	cli, err := httpclient.New(m.cfg.MISTrustAnchor)
	if err != nil {
		c.Reply <- RefreshBundleReply{Err: err}
		return
	}
	res, err := bundle.Fetch(ctx, cli, m.cfg.BundleMapURL, m.state.BundleLastModified)
	if err != nil {
		c.Reply <- RefreshBundleReply{Err: err}
		return
	}

	m.state.LastBundleRefreshAt = m.cfg.Clock.Now().UTC()
	if !res.NotModified {
		m.state.BundleLastModified = res.LastModified
	}
	if err := m.persist(); err != nil {
		c.Reply <- RefreshBundleReply{Err: err}
		return
	}
	c.Reply <- RefreshBundleReply{
		NotModified:     res.NotModified,
		BundleLastModif: m.state.BundleLastModified,
	}
}
