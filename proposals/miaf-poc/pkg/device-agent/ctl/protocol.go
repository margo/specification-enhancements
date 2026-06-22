package ctl

// Request is what `device-agent ctl ...` sends to the daemon socket.
type Request struct {
	Cmd        string   `json:"cmd"` // "status" | "exchange-jwt" | "force-renew" | "force-bundle-refresh" | "force-revocations-check"
	Audiences  []string `json:"audiences,omitempty"`
	TTLSeconds int      `json:"ttlSeconds,omitempty"`
}

// Response is what the daemon writes back. Data is a JSON-encoded handler-
// specific payload (typically the matching identitymanager reply struct).
type Response struct {
	OK    bool        `json:"ok"`
	Error string      `json:"error,omitempty"`
	Data  interface{} `json:"data,omitempty"`
}
