package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request, id string) {
	sticker, err := h.repo.AdminDeleteSticker(r.Context(), id)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, sticker)
}
