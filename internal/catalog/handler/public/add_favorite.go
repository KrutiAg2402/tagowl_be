package public

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleAddFavorite(w http.ResponseWriter, r *http.Request, id string) {
	request := shared.DecodeEventRequest(r)
	response, err := h.repo.AddFavorite(r.Context(), id, shared.ActorKeyFromRequest(r, request.ActorKey))
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, response)
}
