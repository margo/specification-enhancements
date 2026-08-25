package ctl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Call sends req to the daemon socket at path and decodes the response Data
// into out (may be nil). Returns the protocol error from the daemon (if
// Response.OK is false) as a normal Go error.
func Call(ctx context.Context, path string, req Request, out interface{}) error {
	d := net.Dialer{Timeout: 2 * time.Second}
	c, err := d.DialContext(ctx, "unix", path)
	if err != nil {
		return fmt.Errorf("dial daemon socket: %w", err)
	}
	defer c.Close()

	if err := json.NewEncoder(c).Encode(req); err != nil {
		return fmt.Errorf("encode request: %w", err)
	}
	dec := json.NewDecoder(bufio.NewReader(c))
	resp := Response{Data: out}
	if err := dec.Decode(&resp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("daemon: %s", resp.Error)
	}
	return nil
}
