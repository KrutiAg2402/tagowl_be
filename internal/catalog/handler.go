package catalog

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type API struct {
	repo Repository
}

func NewHandler(repo Repository) http.Handler {
	api := &API{repo: repo}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", api.handleHealth)
	mux.HandleFunc("/api/v1/admin/stickers", api.handleAdminStickers)
	mux.HandleFunc("/api/v1/admin/stickers/", api.handleAdminStickerRoutes)
	mux.HandleFunc("/api/v1/home", api.handleHome)
	mux.HandleFunc("/api/v1/stickers", api.handleListStickers)
	mux.HandleFunc("/api/v1/stickers/", api.handleStickerRoutes)
	mux.HandleFunc("/api/v1/orders", api.handleOrders)

	return recoverMiddleware(corsMiddleware(mux))
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"service":   "sticker-catalog-api",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *API) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	limit := parseLimit(r.URL.Query().Get("limit"), defaultHomeLimit)
	response, err := a.repo.Home(r.Context(), limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, response)
}

func (a *API) handleListStickers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	filter := StickerFilter{
		Category: r.URL.Query().Get("category"),
		Tag:      r.URL.Query().Get("tag"),
		Sort:     r.URL.Query().Get("sort"),
		Limit:    parseLimit(r.URL.Query().Get("limit"), defaultListLimit),
	}

	items, err := a.repo.List(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, ListResponse{
		Items:   items,
		Count:   len(items),
		Filters: normalizeFilter(filter),
	})
}

func (a *API) handleStickerRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/stickers/"), "/")
	if path == "" {
		respondError(w, http.StatusBadRequest, "sticker id is required")
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		a.handleGetSticker(w, r, id)
	case len(parts) == 2 && parts[1] == "view" && r.Method == http.MethodPost:
		a.handleRecordView(w, r, id)
	case len(parts) == 2 && parts[1] == "favorite" && r.Method == http.MethodPost:
		a.handleAddFavorite(w, r, id)
	case len(parts) == 2 && parts[1] == "favorite" && r.Method == http.MethodDelete:
		a.handleRemoveFavorite(w, r, id)
	default:
		respondError(w, http.StatusNotFound, "route not found")
	}
}

func (a *API) handleGetSticker(w http.ResponseWriter, r *http.Request, id string) {
	sticker, ok, err := a.repo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !ok {
		respondError(w, http.StatusNotFound, "sticker not found")
		return
	}

	respondJSON(w, http.StatusOK, sticker)
}

func (a *API) handleRecordView(w http.ResponseWriter, r *http.Request, id string) {
	request := decodeEventRequest(r)
	response, err := a.repo.RecordView(r.Context(), id, actorKeyFromRequest(r, request.ActorKey))
	if err != nil {
		writeRepoError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, response)
}

func (a *API) handleAddFavorite(w http.ResponseWriter, r *http.Request, id string) {
	request := decodeEventRequest(r)
	response, err := a.repo.AddFavorite(r.Context(), id, actorKeyFromRequest(r, request.ActorKey))
	if err != nil {
		writeRepoError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, response)
}

func (a *API) handleRemoveFavorite(w http.ResponseWriter, r *http.Request, id string) {
	request := decodeEventRequest(r)
	if request.ActorKey == "" {
		request.ActorKey = r.URL.Query().Get("actorKey")
	}

	response, err := a.repo.RemoveFavorite(r.Context(), id, actorKeyFromRequest(r, request.ActorKey))
	if err != nil {
		writeRepoError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, response)
}

func (a *API) handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var request OrderCreateRequest
	if err := decodeJSON(r, &request); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response, err := a.repo.CreateOrder(r.Context(), request)
	if err != nil {
		writeRepoError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, response)
}

func (a *API) handleAdminStickers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		includeInactive := parseBool(r.URL.Query().Get("includeInactive"))
		items, err := a.repo.AdminList(r.Context(), includeInactive)
		if err != nil {
			writeRepoError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, AdminListResponse{
			Items:           items,
			Count:           len(items),
			IncludeInactive: includeInactive,
		})
	case http.MethodPost:
		var request AdminCreateStickerRequest
		if err := decodeJSON(r, &request); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		sticker, err := a.repo.AdminCreateSticker(r.Context(), request)
		if err != nil {
			writeRepoError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, sticker)
	default:
		methodNotAllowed(w)
	}
}

func (a *API) handleAdminStickerRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/stickers/"), "/")
	if path == "" {
		respondError(w, http.StatusBadRequest, "sticker id is required")
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]

	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		sticker, ok, err := a.repo.AdminGetByID(r.Context(), id)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		if !ok {
			respondError(w, http.StatusNotFound, "sticker not found")
			return
		}
		respondJSON(w, http.StatusOK, sticker)
	case len(parts) == 1 && r.Method == http.MethodPatch:
		var request AdminUpdateStickerRequest
		if err := decodeJSON(r, &request); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		sticker, err := a.repo.AdminUpdateSticker(r.Context(), id, request)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, sticker)
	case len(parts) == 1 && r.Method == http.MethodDelete:
		sticker, err := a.repo.AdminDeleteSticker(r.Context(), id)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, sticker)
	case len(parts) == 2 && parts[1] == "price" && r.Method == http.MethodPatch:
		var request AdminUpdatePriceRequest
		if err := decodeJSON(r, &request); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		sticker, err := a.repo.AdminUpdatePrice(r.Context(), id, request)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, sticker)
	case len(parts) == 2 && parts[1] == "status" && r.Method == http.MethodPatch:
		var request AdminUpdateStatusRequest
		if err := decodeJSON(r, &request); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		sticker, err := a.repo.AdminUpdateStatus(r.Context(), id, request)
		if err != nil {
			writeRepoError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, sticker)
	default:
		respondError(w, http.StatusNotFound, "route not found")
	}
}

func decodeEventRequest(r *http.Request) EventRequest {
	var request EventRequest
	if r.ContentLength == 0 {
		return request
	}
	if err := decodeJSON(r, &request); err != nil {
		return EventRequest{}
	}
	return request
}

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}

	return nil
}

func actorKeyFromRequest(r *http.Request, provided string) string {
	if key := strings.TrimSpace(provided); key != "" {
		return key
	}

	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	return strings.TrimSpace(r.RemoteAddr)
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrStickerNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrActorKeyRequired), errors.Is(err, ErrEmptyOrder), errors.Is(err, ErrInvalidSticker), errors.Is(err, ErrInvalidPrice), errors.Is(err, ErrNoStickerChanges):
		respondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrDuplicateSticker):
		respondError(w, http.StatusConflict, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, err.Error())
	}
}

func parseLimit(value string, fallback int) int {
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func parseBool(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return parsed
}

func respondJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func respondError(w http.ResponseWriter, statusCode int, message string) {
	respondJSON(w, statusCode, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	respondError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %v", recovered)
				respondError(w, http.StatusInternalServerError, "internal server error")
			}
		}()

		next.ServeHTTP(w, r)
	})
}
