package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request, id string) {
	sticker, ok, err := h.repo.AdminGetByID(r.Context(), id)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}
	if !ok {
		shared.RespondError(w, http.StatusNotFound, "sticker not found")
		return
	}

	shared.RespondJSON(w, http.StatusOK, sticker)
}
