package category

import (
	"net/http"

	"tagowl/backend/internal/catalog"
)

type Handler struct {
	repo catalog.Repository
}

func Register(mux *http.ServeMux, repo catalog.Repository) {
	handler := &Handler{repo: repo}

	mux.HandleFunc("/api/v1/admin/categories", handler.handleCategories)
	mux.HandleFunc("/api/v1/admin/categories/", handler.handleCategoryRoutes)
}
