package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/Pesokrava/product_reviewer/internal/delivery/http/request"
	"github.com/Pesokrava/product_reviewer/internal/delivery/http/response"
	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/Pesokrava/product_reviewer/internal/usecase/review"
)

// ReviewHandler handles HTTP requests for reviews
type ReviewHandler struct {
	service *review.Service
	logger  *logger.Logger
}

// NewReviewHandler creates a new review handler
func NewReviewHandler(service *review.Service, log *logger.Logger) *ReviewHandler {
	return &ReviewHandler{
		service: service,
		logger:  log,
	}
}

// CreateReviewRequest represents the request body for creating a review
type CreateReviewRequest struct {
	ProductID  string `json:"product_id" validate:"required"`
	FirstName  string `json:"first_name" validate:"required,min=1,max=100"`
	LastName   string `json:"last_name" validate:"required,min=1,max=100"`
	ReviewText string `json:"review_text" validate:"required,min=1"`
	Rating     int    `json:"rating" validate:"required,min=1,max=5"`
}

// UpdateReviewRequest represents the request body for updating a review
type UpdateReviewRequest struct {
	FirstName  string `json:"first_name" validate:"required,min=1,max=100"`
	LastName   string `json:"last_name" validate:"required,min=1,max=100"`
	ReviewText string `json:"review_text" validate:"required,min=1"`
	Rating     int    `json:"rating" validate:"required,min=1,max=5"`
}

// Create handles POST /api/v1/reviews
// @Summary Create a new review
// @Description Create a new review for a product. Automatically updates product's average rating and publishes event.
// @Tags Reviews
// @Accept json
// @Produce json
// @Param review body CreateReviewRequest true "Review details"
// @Success 201 {object} map[string]interface{} "Review created successfully"
// @Failure 400 {object} map[string]string "Invalid request body or product not found"
// @Failure 404 {object} map[string]string "Product not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /reviews [post]
func (h *ReviewHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateReviewRequest
	if err := request.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	productID, err := uuid.Parse(req.ProductID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	review := &domain.Review{
		ProductID:  productID,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		ReviewText: req.ReviewText,
		Rating:     req.Rating,
	}

	if err := h.service.Create(r.Context(), review); err != nil {
		h.handleError(w, err)
		return
	}

	response.Created(w, review)
}

// Update handles PUT /api/v1/reviews/:id
// @Summary Update a review
// @Description Update review details. Automatically recalculates product's average rating and publishes event.
// @Tags Reviews
// @Accept json
// @Produce json
// @Param id path string true "Review ID (UUID)"
// @Param review body UpdateReviewRequest true "Updated review details"
// @Success 200 {object} map[string]interface{} "Review updated successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Review not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /reviews/{id} [put]
func (h *ReviewHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.GetUUIDParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid review ID")
		return
	}

	var req UpdateReviewRequest
	if err := request.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	review := &domain.Review{
		ID:         id,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		ReviewText: req.ReviewText,
		Rating:     req.Rating,
	}

	if err := h.service.Update(r.Context(), review); err != nil {
		h.handleError(w, err)
		return
	}

	response.Success(w, review)
}

// Delete handles DELETE /api/v1/reviews/:id
// @Summary Delete a review
// @Description Soft delete a review. Automatically recalculates product's average rating and publishes event.
// @Tags Reviews
// @Accept json
// @Produce json
// @Param id path string true "Review ID (UUID)"
// @Success 204 "Review deleted successfully"
// @Failure 400 {object} map[string]string "Invalid review ID"
// @Failure 404 {object} map[string]string "Review not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /reviews/{id} [delete]
func (h *ReviewHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.GetUUIDParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid review ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}

	response.NoContent(w)
}

// GetByProductID handles GET /api/v1/products/:id/reviews
// @Summary Get reviews for a product
// @Description Get a paginated list of reviews for a specific product. Results are cached.
// @Tags Reviews
// @Accept json
// @Produce json
// @Param id path string true "Product ID (UUID)"
// @Param limit query int false "Number of items per page (max 100)" default(20)
// @Param offset query int false "Number of items to skip" default(0)
// @Success 200 {object} map[string]interface{} "Paginated list of reviews"
// @Failure 400 {object} map[string]string "Invalid product ID"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /products/{id}/reviews [get]
func (h *ReviewHandler) GetByProductID(w http.ResponseWriter, r *http.Request) {
	productID, err := request.GetUUIDParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	limit, offset := request.GetPaginationParams(r)

	reviews, total, err := h.service.GetByProductID(r.Context(), productID, limit, offset)
	if err != nil {
		h.handleError(w, err)
		return
	}

	response.Paginated(w, reviews, total, limit, offset)
}

// handleError handles service layer errors and returns appropriate HTTP responses
func (h *ReviewHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		response.Error(w, http.StatusNotFound, "Review or product not found")
	case errors.Is(err, domain.ErrInvalidInput):
		response.Error(w, http.StatusBadRequest, "Invalid input")
	default:
		h.logger.Error("Internal error in review handler", err)
		response.Error(w, http.StatusInternalServerError, "Internal server error")
	}
}
