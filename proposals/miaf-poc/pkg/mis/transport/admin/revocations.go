package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/margo/miaf-poc/pkg/common"
	"github.com/margo/miaf-poc/pkg/mis/identity"
)

func (s *Server) createRevocation(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Revocations == nil {
		writeError(w, http.StatusNotImplemented, "revocation operator not configured")
		return
	}
	var req common.RevokeSPIFFEIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.SPIFFEID == "" {
		writeError(w, http.StatusBadRequest, "spiffe_id is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	newly, already, err := s.cfg.Revocations.RevokeSPIFFEID(ctx, req.SPIFFEID, req.Reason)
	if errors.Is(err, identity.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no identity bound to "+req.SPIFFEID)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "revocation failed")
		return
	}
	newlySerials := make([]string, 0, len(newly))
	for _, e := range newly {
		newlySerials = append(newlySerials, e.SerialHex)
	}
	writeJSON(w, http.StatusOK, common.RevokeSPIFFEIDResponse{
		SPIFFEID:       req.SPIFFEID,
		RevokedSerials: newlySerials,
		AlreadyRevoked: already,
	})
}
