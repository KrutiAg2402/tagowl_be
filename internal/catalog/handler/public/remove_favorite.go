package public

import (
	"net/http"

	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleRemoveFavorite(w http.ResponseWriter, r *http.Request, id string) {
	request := shared.DecodeEventRequest(r)
	if request.ActorKey == "" {
		request.ActorKey = r.URL.Query().Get("actorKey")
	}

	response, err := h.repo.RemoveFavorite(r.Context(), id, shared.ActorKeyFromRequest(r, request.ActorKey))
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, response)
}
