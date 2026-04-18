package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	includeInactive := shared.ParseBool(r.URL.Query().Get("includeInactive"))
	items, err := h.repo.AdminList(r.Context(), includeInactive)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, catalog.AdminListResponse{
		Items:           items,
		Count:           len(items),
		IncludeInactive: includeInactive,
	})
}
