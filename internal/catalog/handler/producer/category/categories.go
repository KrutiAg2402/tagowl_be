package category

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListCategories(w, r)
	case http.MethodPost:
		h.handleCreateCategory(w, r)
	default:
		shared.MethodNotAllowed(w)
	}
}
