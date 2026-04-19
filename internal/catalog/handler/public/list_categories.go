package public

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleListCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		shared.MethodNotAllowed(w)
		return
	}

	items, err := h.repo.ListCategories(r.Context())
	if err != nil {
		shared.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	shared.RespondJSON(w, http.StatusOK, catalog.CategoryListResponse{
		Items: items,
		Count: len(items),
	})
}
