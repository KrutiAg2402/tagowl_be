package public

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleGetSticker(w http.ResponseWriter, r *http.Request, id string) {
	sticker, ok, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		shared.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		shared.RespondError(w, http.StatusNotFound, "sticker not found")
		return
	}

	shared.RespondJSON(w, http.StatusOK, sticker)
}
