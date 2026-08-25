package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/margo/miaf-poc/pkg/device-agent/ctl"
	"github.com/margo/miaf-poc/pkg/device-agent/identitymanager"
)

type socketServer struct {
	path string
	mgr  *identitymanager.Manager
	log  *slog.Logger
	ln   net.Listener
}

func newSocketServer(path string, mgr *identitymanager.Manager, log *slog.Logger) *socketServer {
	return &socketServer{path: path, mgr: mgr, log: log}
}

func (s *socketServer) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	_ = os.Remove(s.path)
	ln, err := net.Listen("unix", s.path)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		_ = ln.Close()
		return err
	}
	s.ln = ln
	go s.accept(ctx)
	return nil
}

func (s *socketServer) Close() {
	if s.ln != nil {
		_ = s.ln.Close()
		_ = os.Remove(s.path)
	}
}

func (s *socketServer) accept(ctx context.Context) {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.log.Warn("socket accept", "err", err)
			return
		}
		go s.handle(ctx, c)
	}
}

func (s *socketServer) handle(ctx context.Context, c net.Conn) {
	defer c.Close()
	dec := json.NewDecoder(bufio.NewReader(c))
	enc := json.NewEncoder(c)

	var req ctl.Request
	if err := dec.Decode(&req); err != nil {
		_ = enc.Encode(ctl.Response{OK: false, Error: "decode request: " + err.Error()})
		return
	}

	switch req.Cmd {
	case "status":
		reply := make(chan identitymanager.StatusReply, 1)
		s.mgr.Submit(identitymanager.Cmd{
			Ctx:    ctx,
			Status: &identitymanager.StatusCmd{Reply: reply},
		})
		r := <-reply
		_ = enc.Encode(ctl.Response{OK: true, Data: r})

	case "exchange-jwt":
		reply := make(chan identitymanager.ExchangeJWTReply, 1)
		s.mgr.Submit(identitymanager.Cmd{
			Ctx: ctx,
			JWT: &identitymanager.ExchangeJWTCmd{
				Audiences: req.Audiences,
				TTL:       time.Duration(req.TTLSeconds) * time.Second,
				Reply:     reply,
			},
		})
		r := <-reply
		if r.Err != nil {
			_ = enc.Encode(ctl.Response{OK: false, Error: r.Err.Error()})
			return
		}
		_ = enc.Encode(ctl.Response{OK: true, Data: r})

	case "force-renew":
		reply := make(chan identitymanager.RenewReply, 1)
		s.mgr.Submit(identitymanager.Cmd{
			Ctx:   ctx,
			Renew: &identitymanager.RenewCmd{Reply: reply},
		})
		r := <-reply
		if r.Err != nil {
			_ = enc.Encode(ctl.Response{OK: false, Error: r.Err.Error()})
			return
		}
		_ = enc.Encode(ctl.Response{OK: true, Data: r})

	case "force-revocations-check":
		reply := make(chan identitymanager.CheckRevocationsReply, 1)
		s.mgr.Submit(identitymanager.Cmd{
			Ctx:   ctx,
			Check: &identitymanager.CheckRevocationsCmd{Reply: reply},
		})
		r := <-reply
		if r.Err != nil {
			_ = enc.Encode(ctl.Response{OK: false, Error: r.Err.Error()})
			return
		}
		_ = enc.Encode(ctl.Response{OK: true, Data: r})

	case "force-bundle-refresh":
		reply := make(chan identitymanager.RefreshBundleReply, 1)
		s.mgr.Submit(identitymanager.Cmd{
			Ctx:     ctx,
			Refresh: &identitymanager.RefreshBundleCmd{Reply: reply},
		})
		r := <-reply
		if r.Err != nil {
			_ = enc.Encode(ctl.Response{OK: false, Error: r.Err.Error()})
			return
		}
		_ = enc.Encode(ctl.Response{OK: true, Data: r})

	default:
		_ = enc.Encode(ctl.Response{OK: false, Error: "unknown cmd: " + req.Cmd})
	}
}
