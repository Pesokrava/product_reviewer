package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"

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
}

// NewRouter creates a new HTTP router
func NewRouter(
	productHandler *handler.ProductHandler,
	reviewHandler *handler.ReviewHandler,
	log *logger.Logger,
) *Router {
	return &Router{
		productHandler: productHandler,
		reviewHandler:  reviewHandler,
		logger:         log,
	}
}

// Setup configures and returns the HTTP router
func (rt *Router) Setup() http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recovery(rt.logger))
	r.Use(middleware.Logger(rt.logger))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", rt.healthCheck)

	// Swagger documentation
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Product routes
		r.Route("/products", func(r chi.Router) {
			r.Post("/", rt.productHandler.Create)
			r.Get("/", rt.productHandler.List)
			r.Get("/{id}", rt.productHandler.GetByID)
			r.Put("/{id}", rt.productHandler.Update)
			r.Delete("/{id}", rt.productHandler.Delete)
			r.Get("/{id}/reviews", rt.reviewHandler.GetByProductID)
		})

		// Review routes
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
