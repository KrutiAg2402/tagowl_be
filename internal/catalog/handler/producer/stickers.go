package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleStickers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleList(w, r)
	case http.MethodPost:
		h.handleCreate(w, r)
	default:
		shared.MethodNotAllowed(w)
	}
}
