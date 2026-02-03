package handler

import (
	"errors"
	"net/http"

	"github.com/Pesokrava/product_reviewer/internal/delivery/http/request"
	"github.com/Pesokrava/product_reviewer/internal/delivery/http/response"
	"github.com/Pesokrava/product_reviewer/internal/domain"
	"github.com/Pesokrava/product_reviewer/internal/pkg/logger"
	"github.com/Pesokrava/product_reviewer/internal/usecase/product"
)

// ProductHandler handles HTTP requests for products
type ProductHandler struct {
	service *product.Service
	logger  *logger.Logger
}

// NewProductHandler creates a new product handler
func NewProductHandler(service *product.Service, log *logger.Logger) *ProductHandler {
	return &ProductHandler{
		service: service,
		logger:  log,
	}
}

// CreateProductRequest represents the request body for creating a product
type CreateProductRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty"`
	Price       float64 `json:"price" validate:"required,gte=0"`
}

// UpdateProductRequest represents the request body for updating a product
type UpdateProductRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty"`
	Price       float64 `json:"price" validate:"required,gte=0"`
}

// Create handles POST /api/v1/products
// @Summary Create a new product
// @Description Create a new product with name, description, and price
// @Tags Products
// @Accept json
// @Produce json
// @Param product body CreateProductRequest true "Product details"
// @Success 201 {object} map[string]interface{} "Product created successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /products [post]
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateProductRequest
	if err := request.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	product := &domain.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
	}

	if err := h.service.Create(r.Context(), product); err != nil {
		h.handleError(w, err)
		return
	}

	response.Created(w, product)
}

// GetByID handles GET /api/v1/products/:id
// @Summary Get a product by ID
// @Description Get detailed information about a product including average rating
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "Product ID (UUID)"
// @Success 200 {object} map[string]interface{} "Product details"
// @Failure 400 {object} map[string]string "Invalid product ID"
// @Failure 404 {object} map[string]string "Product not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /products/{id} [get]
func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := request.GetUUIDParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	product, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	response.Success(w, product)
}

// List handles GET /api/v1/products
// @Summary List all products
// @Description Get a paginated list of products
// @Tags Products
// @Accept json
// @Produce json
// @Param limit query int false "Number of items per page (max 100)" default(20)
// @Param offset query int false "Number of items to skip" default(0)
// @Success 200 {object} map[string]interface{} "Paginated list of products"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /products [get]
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := request.GetPaginationParams(r)

	products, total, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		h.handleError(w, err)
		return
	}

	response.Paginated(w, products, total, limit, offset)
}

// Update handles PUT /api/v1/products/:id
// @Summary Update a product
// @Description Update product details (name, description, price)
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "Product ID (UUID)"
// @Param product body UpdateProductRequest true "Updated product details"
// @Success 200 {object} map[string]interface{} "Product updated successfully"
// @Failure 400 {object} map[string]string "Invalid request"
// @Failure 404 {object} map[string]string "Product not found"
// @Failure 409 {object} map[string]string "Conflict - product was modified"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /products/{id} [put]
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.GetUUIDParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	var req UpdateProductRequest
	if err := request.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Version field required for optimistic locking but not provided in update request
	existingProduct, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	product := &domain.Product{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Version:     existingProduct.Version,
	}

	if err := h.service.Update(r.Context(), product); err != nil {
		h.handleError(w, err)
		return
	}

	response.Success(w, product)
}

// Delete handles DELETE /api/v1/products/:id
// @Summary Delete a product
// @Description Soft delete a product and all its reviews
// @Tags Products
// @Accept json
// @Produce json
// @Param id path string true "Product ID (UUID)"
// @Success 204 "Product deleted successfully"
// @Failure 400 {object} map[string]string "Invalid product ID"
// @Failure 404 {object} map[string]string "Product not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /products/{id} [delete]
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.GetUUIDParam(r, "id")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid product ID")
		return
	}

	if err := h.service.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}

	response.NoContent(w)
}

// handleError handles service layer errors and returns appropriate HTTP responses
func (h *ProductHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		response.Error(w, http.StatusNotFound, "Product not found")
	case errors.Is(err, domain.ErrInvalidInput):
		response.Error(w, http.StatusBadRequest, "Invalid input")
	case errors.Is(err, domain.ErrConflict):
		response.Error(w, http.StatusConflict, "Conflict - product was modified by another request")
	default:
		h.logger.Error("Internal error in product handler", err)
		response.Error(w, http.StatusInternalServerError, "Internal server error")
	}
}
