package public

import (
	"net/http"
	"strings"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleStickerRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/stickers/"), "/")
	if path == "" {
		shared.RespondError(w, http.StatusBadRequest, "sticker id is required")
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		h.handleGetSticker(w, r, id)
	case len(parts) == 2 && parts[1] == "view" && r.Method == http.MethodPost:
		h.handleRecordView(w, r, id)
	case len(parts) == 2 && parts[1] == "favorite" && r.Method == http.MethodPost:
		h.handleAddFavorite(w, r, id)
	case len(parts) == 2 && parts[1] == "favorite" && r.Method == http.MethodDelete:
		h.handleRemoveFavorite(w, r, id)
	default:
		shared.RespondError(w, http.StatusNotFound, "route not found")
	}
}
