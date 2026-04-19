package category

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleGetCategory(w http.ResponseWriter, r *http.Request, id string) {
	category, ok, err := h.repo.AdminGetCategoryByID(r.Context(), id)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}
	if !ok {
		shared.RespondError(w, http.StatusNotFound, "category not found")
		return
	}

	shared.RespondJSON(w, http.StatusOK, category)
}
