package ctl_test

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"testing"

	"github.com/margo/miaf-poc/pkg/device-agent/ctl"
)

func TestCtl_StatusRoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sock := filepath.Join(dir, "d.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		defer c.Close()
		var req ctl.Request
		_ = json.NewDecoder(c).Decode(&req)
		_ = json.NewEncoder(c).Encode(ctl.Response{OK: true, Data: map[string]string{"spiffe_id": "spiffe://td/x"}})
	}()

	var out map[string]string
	if err := ctl.Call(context.Background(), sock, ctl.Request{Cmd: "status"}, &out); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out["spiffe_id"] != "spiffe://td/x" {
		t.Errorf("got %v", out)
	}
}

func TestCtl_ErrorPropagated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sock := filepath.Join(dir, "err.sock")
	ln, _ := net.Listen("unix", sock)
	defer ln.Close()

	go func() {
		c, _ := ln.Accept()
		defer c.Close()
		_ = json.NewEncoder(c).Encode(ctl.Response{OK: false, Error: "unknown cmd: foo"})
	}()

	err := ctl.Call(context.Background(), sock, ctl.Request{Cmd: "foo"}, nil)
	if err == nil {
		t.Fatal("expected error for OK=false response")
	}
}
