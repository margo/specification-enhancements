package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
)

func (s *Server) createEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	var req common.CreateEnrollmentTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var ttl time.Duration
	switch {
	case req.TTL != "":
		d, err := time.ParseDuration(req.TTL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid ttl: "+err.Error())
			return
		}
		ttl = d
	case s.cfg.DefaultTokenTTL > 0:
		ttl = s.cfg.DefaultTokenTTL
	default:
		writeError(w, http.StatusBadRequest, "ttl is required (no server default configured)")
		return
	}
	if ttl <= 0 {
		writeError(w, http.StatusBadRequest, "ttl must be positive")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	c, err := s.cfg.EnrollmentTokens.Create(ctx, ttl, time.Now())
	if err != nil {
		s.cfg.Logger.Error("admin: create enrollment token", "error", err)
		writeError(w, http.StatusInternalServerError, "create token failed")
		return
	}
	writeJSON(w, http.StatusCreated, common.CreateEnrollmentTokenResponse{
		TokenID:   c.TokenID,
		Secret:    c.Secret.Wire(),
		ExpiresAt: c.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}
