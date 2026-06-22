package http

import (
	"encoding/json"
	"net/http"

	"github.com/margo/miaf-poc/pkg/common"
)

// NewDiscoveryHandler returns an http.Handler that serves the Margo
// discovery document per SUP §5. The document is static; rotation is a
// future concern.
func NewDiscoveryHandler(doc *common.DiscoveryResponseDTO) http.Handler {
	// Pre-marshal so every request is a single buffered write.
	body, err := json.Marshal(doc)
	if err != nil {
		panic("http: discovery document failed to marshal: " + err.Error())
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}
