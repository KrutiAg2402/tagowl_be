package category

import (
	"net/http"
	"strings"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleCategoryRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/categories/"), "/")
	if path == "" {
		shared.RespondError(w, http.StatusBadRequest, "category id is required")
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		h.handleGetCategory(w, r, id)
	case len(parts) == 1 && r.Method == http.MethodPatch:
		h.handleUpdateCategory(w, r, id)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		h.handleDeleteCategory(w, r, id)
	case len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodPatch:
		h.handleUpdateCategoryStatus(w, r, id)
	default:
		shared.RespondError(w, http.StatusNotFound, "route not found")
	}
}
