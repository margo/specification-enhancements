package identitymanager

import (
	"context"
	"fmt"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
	jwtcache "github.com/margo/miaf-poc/pkg/device-agent/jwtsvid/cache"
	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
)

func (m *Manager) handleExchangeJWT(ctx context.Context, c *ExchangeJWTCmd) {
	defer close(c.Reply)
	if m.revoked {
		c.Reply <- ExchangeJWTReply{Err: fmt.Errorf("device is revoked; refusing to exchange")}
		return
	}

	now := m.cfg.Clock.Now().UTC()
	if m.cfg.JWTCache != nil {
		if cached, ok := m.cfg.JWTCache.Get(c.Audiences, now); ok {
			c.Reply <- ExchangeJWTReply{
				JWT:       cached.Token,
				ExpiresAt: cached.ExpiresAt,
				FromCache: true,
			}
			return
		}
	}

	cli, err := httpclient.NewMTLS(m.cfg.MISTrustAnchor, m.cfg.SVIDCertFile, m.cfg.SVIDKeyFile)
	if err != nil {
		c.Reply <- ExchangeJWTReply{Err: err}
		return
	}
	signer, err := keygen.LoadSigner(m.cfg.SVIDKeyFile)
	if err != nil {
		c.Reply <- ExchangeJWTReply{Err: err}
		return
	}
	res, err := jwtsvid.Exchange(ctx, jwtsvid.Request{
		Client:    cli,
		MISBaseURL: m.cfg.MISBaseURL,
		SPIFFEID:  m.state.SPIFFEID,
		Audiences: c.Audiences,
		TTL:       c.TTL,
		Mode:      jwtsvid.ModeMTLS,
		SigningKey: signer,
	})
	if err != nil {
		c.Reply <- ExchangeJWTReply{Err: err}
		return
	}
	if res.HTTPStatus < 200 || res.HTTPStatus >= 300 {
		c.Reply <- ExchangeJWTReply{
			HTTPStatus: res.HTTPStatus,
			Problem:    res.Problem,
			Err:        fmt.Errorf("jwt-svid returned %d: %s", res.HTTPStatus, res.Problem.Title),
		}
		return
	}

	expiry, _ := time.Parse(time.RFC3339, res.Response.ExpiresAt)
	if m.cfg.JWTCache != nil {
		m.cfg.JWTCache.Put(c.Audiences, jwtcache.Entry{
			Token:     res.Response.JWT,
			ExpiresAt: expiry,
		})
	}
	c.Reply <- ExchangeJWTReply{
		HTTPStatus: res.HTTPStatus,
		JWT:        res.Response.JWT,
		ExpiresAt:  expiry,
		FromCache:  false,
	}
}
