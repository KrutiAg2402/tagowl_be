package public

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleListStickers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		shared.MethodNotAllowed(w)
		return
	}

	filter := catalog.StickerFilter{
		Category: r.URL.Query().Get("category"),
		Tag:      r.URL.Query().Get("tag"),
		Sort:     r.URL.Query().Get("sort"),
		Limit:    shared.ParseLimit(r.URL.Query().Get("limit"), catalog.DefaultListLimit),
	}

	items, err := h.repo.List(r.Context(), filter)
	if err != nil {
		shared.RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	shared.RespondJSON(w, http.StatusOK, catalog.ListResponse{
		Items:   items,
		Count:   len(items),
		Filters: catalog.NormalizeFilter(filter),
	})
}
