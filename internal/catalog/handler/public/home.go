package public

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		shared.MethodNotAllowed(w)
		return
	}

	limit := shared.ParseLimit(r.URL.Query().Get("limit"), catalog.DefaultHomeLimit)
	response, err := h.repo.Home(r.Context(), limit)
	if err != nil {
		shared.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	shared.RespondJSON(w, http.StatusOK, response)
}
