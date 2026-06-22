package identitymanager

import "context"

func (m *Manager) handleStatus(_ context.Context, c *StatusCmd) {
	r := StatusReply{
		SPIFFEID:                m.state.SPIFFEID,
		Serial:                  m.state.Serial,
		ExpiresAt:               m.state.ExpiresAt,
		LastRenewalAt:           m.state.LastRenewalAt,
		LastBundleRefreshAt:     m.state.LastBundleRefreshAt,
		LastRevocationsCheckAt:  m.state.LastRevocationsCheckAt,
		RevokedAt:               m.state.RevokedAt,
		BundleLastModified:      m.state.BundleLastModified,
		RevocationsETag:         m.state.RevocationsETag,
		RevocationsLastModified: m.state.RevocationsLastModified,
	}
	if m.cfg.JWTCache != nil {
		r.JWTCacheSize = m.cfg.JWTCache.Len()
	}
	c.Reply <- r
	close(c.Reply)
}
