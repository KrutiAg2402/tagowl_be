package handler

import (
	"net/http"

	"tagowl/backend/internal/catalog"
	"tagowl/backend/internal/catalog/handler/producer"
	"tagowl/backend/internal/catalog/handler/public"
	"tagowl/backend/internal/catalog/handler/shared"
)

func New(repo catalog.Repository) http.Handler {
	mux := http.NewServeMux()

	public.Register(mux, repo)
	producer.Register(mux, repo)

	return shared.RecoverMiddleware(shared.CORSMiddleware(mux))
}
