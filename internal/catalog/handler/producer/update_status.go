package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleUpdateStatus(w http.ResponseWriter, r *http.Request, id string) {
	var request catalog.AdminUpdateStatusRequest
	if err := shared.DecodeJSON(r, &request); err != nil {
		shared.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	sticker, err := h.repo.AdminUpdateStatus(r.Context(), id, request)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, sticker)
}
