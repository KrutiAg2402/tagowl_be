package category

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleUpdateCategoryStatus(w http.ResponseWriter, r *http.Request, id string) {
	var request catalog.AdminUpdateCategoryStatusRequest
	if err := shared.DecodeJSON(r, &request); err != nil {
		shared.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	category, err := h.repo.AdminUpdateCategoryStatus(r.Context(), id, request)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, category)
}
