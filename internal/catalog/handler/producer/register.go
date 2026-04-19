package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	categoryhandler "tagowl/backend/internal/catalog/handler/producer/category"
)

type Handler struct {
	repo catalog.Repository
}

func Register(mux *http.ServeMux, repo catalog.Repository) {
	handler := &Handler{repo: repo}

	categoryhandler.Register(mux, repo)
	mux.HandleFunc("/api/v1/admin/stickers", handler.handleStickers)
	mux.HandleFunc("/api/v1/admin/stickers/", handler.handleStickerRoutes)
}
