package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"

	"github.com/Pesokrava/product_reviewer/internal/config"
	"github.com/Pesokrava/product_reviewer/internal/delivery/http/handler"
	"github.com/Pesokrava/product_reviewer/internal/delivery/http/middleware"
	"github.com/Pesokrava/product_reviewer/internal/delivery/http/response"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
)

// Router holds HTTP handlers and router configuration
type Router struct {
	productHandler *handler.ProductHandler
	reviewHandler  *handler.ReviewHandler
	logger         *logger.Logger
	cfg            *config.Config
}

// NewRouter creates a new HTTP router
func NewRouter(
	productHandler *handler.ProductHandler,
	reviewHandler *handler.ReviewHandler,
	cfg *config.Config,
	log *logger.Logger,
) *Router {
	return &Router{
		productHandler: productHandler,
		reviewHandler:  reviewHandler,
		logger:         log,
		cfg:            cfg,
	}
}

// Setup configures and returns the HTTP router
func (rt *Router) Setup() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recovery(rt.logger))
	r.Use(middleware.Logger(rt.logger))
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", rt.healthCheck)
	// Redirect /docs to /docs/index.html to ensure the Swagger UI is served correctly
	r.Get("/docs", http.RedirectHandler("/docs/index.html", http.StatusMovedPermanently).ServeHTTP)
	r.Get("/docs/*", httpSwagger.WrapHandler)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/products", func(r chi.Router) {
			r.Post("/", rt.productHandler.Create)
			r.Get("/", rt.productHandler.List)
			r.Get("/{id}", rt.productHandler.GetByID)
			r.Put("/{id}", rt.productHandler.Update)
			r.Delete("/{id}", rt.productHandler.Delete)
			r.Get("/{id}/reviews", rt.reviewHandler.GetByProductID)
		})

		r.Route("/reviews", func(r chi.Router) {
			r.Post("/", rt.reviewHandler.Create)
			r.Put("/{id}", rt.reviewHandler.Update)
			r.Delete("/{id}", rt.reviewHandler.Delete)
		})
	})

	return r
}

// healthCheck handles health check requests
func (rt *Router) healthCheck(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}
