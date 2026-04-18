package public

import (
	"net/http"

	"tagowl/backend/internal/catalog"
)

type Handler struct {
	repo catalog.Repository
}

func Register(mux *http.ServeMux, repo catalog.Repository) {
	handler := &Handler{repo: repo}

	mux.HandleFunc("/healthz", handler.handleHealth)
	mux.HandleFunc("/api/v1/home", handler.handleHome)
	mux.HandleFunc("/api/v1/stickers", handler.handleListStickers)
	mux.HandleFunc("/api/v1/stickers/", handler.handleStickerRoutes)
	mux.HandleFunc("/api/v1/orders", handler.handleOrders)
}
