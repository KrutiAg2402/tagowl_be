package public

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		shared.MethodNotAllowed(w)
		return
	}

	var request catalog.OrderCreateRequest
	if err := shared.DecodeJSON(r, &request); err != nil {
		shared.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := h.repo.CreateOrder(r.Context(), request)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusCreated, response)
}
