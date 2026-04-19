package category

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleDeleteCategory(w http.ResponseWriter, r *http.Request, id string) {
	category, err := h.repo.AdminDeleteCategory(r.Context(), id)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, category)
}
