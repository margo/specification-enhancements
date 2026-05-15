package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
)

func (s *Server) createReplacementTicket(w http.ResponseWriter, r *http.Request) {
	var req common.CreateReplacementTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.SPIFFEID == "" {
		writeError(w, http.StatusBadRequest, "spiffe_id is required")
		return
	}
	if req.TTL == "" {
		writeError(w, http.StatusBadRequest, "ttl is required")
		return
	}
	ttl, err := time.ParseDuration(req.TTL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ttl: "+err.Error())
		return
	}
	if ttl <= 0 {
		writeError(w, http.StatusBadRequest, "ttl must be positive")
		return
	}
	ldiUUID := path.Base(req.SPIFFEID)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	c, err := s.cfg.ReplacementTickets.Create(ctx, ldiUUID, ttl, time.Now())
	if err != nil {
		// The store returns "no such ldi" as a plain error; surface
		// unknown-LDI as 400, anything else as 500.
		if strings.Contains(err.Error(), "no such ldi") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.cfg.Logger.Error("admin: create replacement ticket", "error", err)
		writeError(w, http.StatusInternalServerError, "create ticket failed")
		return
	}
	writeJSON(w, http.StatusCreated, common.CreateReplacementTicketResponse{
		Ticket:    c.Ticket,
		SPIFFEID:  req.SPIFFEID,
		ExpiresAt: c.ExpiresAt.UTC().Format(time.RFC3339Nano),
	})
}
