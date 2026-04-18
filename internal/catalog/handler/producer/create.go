package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var request catalog.AdminCreateStickerRequest
	if err := shared.DecodeJSON(r, &request); err != nil {
		shared.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	sticker, err := h.repo.AdminCreateSticker(r.Context(), request)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusCreated, sticker)
}
