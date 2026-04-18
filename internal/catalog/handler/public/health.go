package public

import (
	"net/http"
	"time"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		shared.MethodNotAllowed(w)
		return
	}

	shared.RespondJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"service":   "sticker-catalog-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
