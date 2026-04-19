package category

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleCreateCategory(w http.ResponseWriter, r *http.Request) {
	var request catalog.AdminCreateCategoryRequest
	if err := shared.DecodeJSON(r, &request); err != nil {
		shared.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	category, err := h.repo.AdminCreateCategory(r.Context(), request)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusCreated, category)
}
