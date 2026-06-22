package identitymanager

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"

	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	"github.com/margo/miaf-poc/pkg/device-agent/revocations"
)

func (m *Manager) handleCheckRevocations(ctx context.Context, c *CheckRevocationsCmd) {
	defer close(c.Reply)

	cli, err := httpclient.New(m.cfg.MISTrustAnchor)
	if err != nil {
		c.Reply <- CheckRevocationsReply{Err: err}
		return
	}
	res, err := revocations.Fetch(ctx, cli, m.cfg.RevocationsURL,
		m.state.RevocationsETag, m.state.RevocationsLastModified)
	if err != nil {
		c.Reply <- CheckRevocationsReply{Err: err}
		return
	}

	m.state.LastRevocationsCheckAt = m.cfg.Clock.Now().UTC()
	if res.HTTPStatus >= 200 && res.HTTPStatus < 300 && !res.NotModified {
		m.state.RevocationsETag = res.ETag
		m.state.RevocationsLastModified = res.LastModified
	}

	revokedNow := false
	if !res.NotModified && res.HTTPStatus == 200 {
		raw, err := os.ReadFile(m.cfg.SVIDCertFile)
		if err != nil {
			c.Reply <- CheckRevocationsReply{Err: err}
			return
		}
		block, _ := pem.Decode(raw)
		if block != nil {
			leaf, _ := x509.ParseCertificate(block.Bytes)
			if leaf != nil {
				fp := sha256.Sum256(leaf.Raw)
				revokedNow = res.ContainsFingerprint(hex.EncodeToString(fp[:]))
			}
		}
	}

	if revokedNow && m.state.RevokedAt.IsZero() {
		m.state.RevokedAt = m.cfg.Clock.Now().UTC()
		m.revoked = true
		m.cfg.Logger.Warn("manager: own SVID is revoked; halting scheduled jobs",
			"serial", m.state.Serial)
	}
	if err := m.persist(); err != nil {
		c.Reply <- CheckRevocationsReply{Err: err}
		return
	}
	c.Reply <- CheckRevocationsReply{
		HTTPStatus:    res.HTTPStatus,
		NotModified:   res.NotModified,
		OwnSVIDListed: revokedNow,
		ETag:          m.state.RevocationsETag,
		LastModified:  m.state.RevocationsLastModified,
	}
}
