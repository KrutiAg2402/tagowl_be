package producer

import (
	"net/http"

	"tagowl/backend/internal/catalog"
)

type Handler struct {
	repo catalog.Repository
}

func Register(mux *http.ServeMux, repo catalog.Repository) {
	handler := &Handler{repo: repo}

	mux.HandleFunc("/api/v1/admin/stickers", handler.handleStickers)
	mux.HandleFunc("/api/v1/admin/stickers/", handler.handleStickerRoutes)
}
