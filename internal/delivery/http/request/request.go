package request

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxRequestBodySize = 1 << 20 // 1MB

// DecodeJSON decodes JSON request body into the provided struct with size limit
func DecodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()

	// Limit request body size to prevent DoS attacks
	limitedReader := io.LimitReader(r.Body, maxRequestBodySize)

	if err := json.NewDecoder(limitedReader).Decode(v); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}
	return nil
}

// GetUUIDParam extracts a UUID parameter from the URL
func GetUUIDParam(r *http.Request, key string) (uuid.UUID, error) {
	param := chi.URLParam(r, key)
	if param == "" {
		return uuid.Nil, fmt.Errorf("missing parameter: %s", key)
	}

	id, err := uuid.Parse(param)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid UUID: %w", err)
	}

	return id, nil
}

// GetIntQuery extracts an integer query parameter with a default value
func GetIntQuery(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

// GetPaginationParams extracts and validates pagination parameters
func GetPaginationParams(r *http.Request) (limit, offset int) {
	limit = GetIntQuery(r, "limit", 20)
	offset = GetIntQuery(r, "offset", 0)

	// Validate and clamp values
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	return limit, offset
}
