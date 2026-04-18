package shared

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"tagowl/backend/internal/catalog"
)

func DecodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		return err
	}

	return nil
}

func DecodeEventRequest(r *http.Request) catalog.EventRequest {
	var request catalog.EventRequest
	if r.ContentLength == 0 {
		return request
	}
	if err := DecodeJSON(r, &request); err != nil {
		return catalog.EventRequest{}
	}
	return request
}

func ActorKeyFromRequest(r *http.Request, provided string) string {
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

func ParseLimit(value string, fallback int) int {
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func ParseBool(value string) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false
	}
	return parsed
}
