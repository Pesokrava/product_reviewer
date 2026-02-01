package response

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// JSON writes a JSON response with proper error handling
func JSON(w http.ResponseWriter, statusCode int, data interface{}) {
	// Buffer JSON encoding to handle errors before writing headers
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		// Can still send proper error response since headers not sent yet
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		// Nothing we can do if encoding the error message itself fails
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to encode response"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// If writing to response fails, connection is broken and no recovery possible
	_, _ = buf.WriteTo(w)
}

// Error writes an error response
func Error(w http.ResponseWriter, statusCode int, message string) {
	JSON(w, statusCode, map[string]string{
		"error": message,
	})
}

// Success writes a success response with data
func Success(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// Created writes a created response
func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

// NoContent writes a no content response
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Paginated writes a paginated response
func Paginated(w http.ResponseWriter, data interface{}, total, limit, offset int) {
	JSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data,
		"pagination": map[string]int{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}
