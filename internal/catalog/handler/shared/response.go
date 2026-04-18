package shared

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"tagowl/backend/internal/catalog"
)

func RespondJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func RespondError(w http.ResponseWriter, statusCode int, message string) {
	RespondJSON(w, statusCode, map[string]string{"error": message})
}

func MethodNotAllowed(w http.ResponseWriter) {
	RespondError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func WriteRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, catalog.ErrStickerNotFound):
		RespondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, catalog.ErrActorKeyRequired), errors.Is(err, catalog.ErrEmptyOrder), errors.Is(err, catalog.ErrInvalidSticker), errors.Is(err, catalog.ErrInvalidPrice), errors.Is(err, catalog.ErrNoStickerChanges):
		RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, catalog.ErrDuplicateSticker):
		RespondError(w, http.StatusConflict, err.Error())
	default:
		RespondError(w, http.StatusInternalServerError, err.Error())
	}
}
