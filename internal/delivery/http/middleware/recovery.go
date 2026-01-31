package middleware

import (
	"net/http"

	"github.com/Pesokrava/product_reviewer/internal/delivery/http/response"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

// Recovery returns a middleware that recovers from panics
func Recovery(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.WithFields(map[string]interface{}{
						"panic":  rec,
						"method": r.Method,
						"path":   r.URL.Path,
					}).Info("Panic recovered")

					response.Error(w, http.StatusInternalServerError, "Internal server error")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
