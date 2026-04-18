package producer

import (
	"net/http"
	"strings"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleStickerRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/stickers/"), "/")
	if path == "" {
		shared.RespondError(w, http.StatusBadRequest, "sticker id is required")
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		h.handleGet(w, r, id)
	case len(parts) == 1 && r.Method == http.MethodPatch:
		h.handleUpdate(w, r, id)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		h.handleDelete(w, r, id)
	case len(parts) == 2 && parts[1] == "price" && r.Method == http.MethodPatch:
		h.handleUpdatePrice(w, r, id)
	case len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodPatch:
		h.handleUpdateStatus(w, r, id)
	default:
		shared.RespondError(w, http.StatusNotFound, "route not found")
	}
}
