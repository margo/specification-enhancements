package identitymanager

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/httpclient"
	"github.com/margo/miaf-poc/pkg/device-agent/jwtsvid"
	"github.com/margo/miaf-poc/pkg/device-agent/keygen"
	"github.com/margo/miaf-poc/pkg/device-agent/renew"
)

func (m *Manager) handleRenew(ctx context.Context, c *RenewCmd) {
	defer close(c.Reply)
	if m.revoked {
		c.Reply <- RenewReply{Err: fmt.Errorf("device is revoked; not renewing")}
		return
	}

	devKey, err := keygen.Generate(keygen.ES256)
	if err != nil {
		c.Reply <- RenewReply{Err: fmt.Errorf("keygen: %w", err)}
		return
	}

	var cli *http.Client
	bearerToken := ""
	switch mode := strings.ToLower(m.cfg.RenewAuth); mode {
	case "", "mtls":
		cli, err = httpclient.NewMTLS(m.cfg.MISTrustAnchor, m.cfg.SVIDCertFile, m.cfg.SVIDKeyFile)
		if err != nil {
			c.Reply <- RenewReply{Err: fmt.Errorf("mtls client: %w", err)}
			return
		}
	case "bearer":
		cli, err = httpclient.New(m.cfg.MISTrustAnchor)
		if err != nil {
			c.Reply <- RenewReply{Err: fmt.Errorf("https client: %w", err)}
			return
		}
		signer, err := keygen.LoadSigner(m.cfg.SVIDKeyFile)
		if err != nil {
			c.Reply <- RenewReply{Err: fmt.Errorf("load current signer: %w", err)}
			return
		}
		svidChain, err := jwtsvid.LoadSVIDChain(m.cfg.SVIDCertFile)
		if err != nil {
			c.Reply <- RenewReply{Err: fmt.Errorf("load svid chain: %w", err)}
			return
		}
		exchange, err := jwtsvid.Exchange(ctx, jwtsvid.Request{
			Client:     cli,
			MISBaseURL: m.cfg.MISBaseURL,
			SPIFFEID:   m.state.SPIFFEID,
			Audiences:  []string{renew.RenewalURL(m.cfg.MISBaseURL, m.state.SPIFFEID)},
			TTL:        5 * time.Minute,
			Mode:       jwtsvid.ModeClientAssertion,
			SigningKey: signer,
			SVIDChain:  svidChain,
		})
		if err != nil {
			c.Reply <- RenewReply{Err: fmt.Errorf("exchange renewal bearer jwt: %w", err)}
			return
		}
		if exchange.HTTPStatus < 200 || exchange.HTTPStatus >= 300 {
			c.Reply <- RenewReply{
				HTTPStatus: exchange.HTTPStatus,
				Problem:    exchange.Problem,
				Err:        fmt.Errorf("renewal bearer exchange returned %d: %s", exchange.HTTPStatus, exchange.Problem.Title),
			}
			return
		}
		bearerToken = exchange.Response.JWT
	default:
		c.Reply <- RenewReply{Err: fmt.Errorf("unsupported renewal auth mode %q", m.cfg.RenewAuth)}
		return
	}

	res, err := renew.Renew(ctx, renew.Request{
		Client:        cli,
		MISBaseURL:    m.cfg.MISBaseURL,
		SPIFFEID:      m.state.SPIFFEID,
		DevicePrivKey: devKey,
		BearerToken:   bearerToken,
	})
	if err != nil {
		c.Reply <- RenewReply{Err: err}
		return
	}
	if res.HTTPStatus < 200 || res.HTTPStatus >= 300 {
		c.Reply <- RenewReply{
			HTTPStatus: res.HTTPStatus,
			Problem:    res.Problem,
			Err:        fmt.Errorf("renewal returned %d: %s", res.HTTPStatus, res.Problem.Title),
		}
		return
	}

	chain := strings.Join(res.Response.SVID.CertificateChainPEM, "")
	leafBlock, _ := pem.Decode([]byte(chain))
	if leafBlock == nil {
		c.Reply <- RenewReply{Err: fmt.Errorf("no PEM block in renewal response chain")}
		return
	}
	leaf, err := x509.ParseCertificate(leafBlock.Bytes)
	if err != nil {
		c.Reply <- RenewReply{Err: fmt.Errorf("parse leaf: %w", err)}
		return
	}

	keyDER, err := x509.MarshalPKCS8PrivateKey(devKey)
	if err != nil {
		c.Reply <- RenewReply{Err: fmt.Errorf("marshal key: %w", err)}
		return
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	if err := writeFile(m.cfg.SVIDCertFile, []byte(chain)); err != nil {
		c.Reply <- RenewReply{Err: err}
		return
	}
	if err := writeFile(m.cfg.SVIDKeyFile, keyPEM); err != nil {
		c.Reply <- RenewReply{Err: err}
		return
	}
	m.state.Serial = leaf.SerialNumber.Text(10)
	m.state.ExpiresAt = leaf.NotAfter.UTC()
	m.state.LastRenewalAt = m.cfg.Clock.Now().UTC()
	if err := m.persist(); err != nil {
		c.Reply <- RenewReply{Err: err}
		return
	}

	c.Reply <- RenewReply{
		HTTPStatus: res.HTTPStatus,
		NewSerial:  m.state.Serial,
		NewExpiry:  m.state.ExpiresAt,
	}
}
