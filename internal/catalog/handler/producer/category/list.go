package category

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/shared"
)

func (h *Handler) handleListCategories(w http.ResponseWriter, r *http.Request) {
	includeInactive := shared.ParseBool(r.URL.Query().Get("includeInactive"))
	pagination := catalog.NormalizePagination(
		shared.ParsePage(r.URL.Query().Get("page"), catalog.DefaultAdminPage),
		shared.ParseLimit(r.URL.Query().Get("limit"), catalog.DefaultAdminLimit),
	)

	items, total, err := h.repo.AdminListCategories(r.Context(), includeInactive, pagination)
	if err != nil {
		shared.WriteRepoError(w, err)
		return
	}

	shared.RespondJSON(w, http.StatusOK, catalog.AdminCategoryListResponse{
		Items:           items,
		Pagination:      catalog.NewPaginationResponse(pagination, len(items), total),
		IncludeInactive: includeInactive,
	})
}
