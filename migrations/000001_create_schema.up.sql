-- ============================================================================
-- Create Product Reviews Database Schema
-- ============================================================================
-- This migration creates the complete schema for the product reviews system.
-- Rating calculation is handled by the rating-worker service via NATS events.
-- ============================================================================

-- Create products table
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL CHECK (price >= 0),
    average_rating DECIMAL(2, 1) DEFAULT 0 CHECK (average_rating >= 0 AND average_rating <= 5),
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Create reviews table
CREATE TABLE IF NOT EXISTS reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    review_text TEXT NOT NULL,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- ============================================================================
-- Indexes for Products Table
-- ============================================================================

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);

-- Index on name for search queries
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);

-- Composite index for products list query
-- Covers WHERE deleted_at IS NULL ORDER BY created_at DESC efficiently
CREATE INDEX IF NOT EXISTS idx_products_deleted_at_created_at
ON products(deleted_at, created_at DESC)
WHERE deleted_at IS NULL;

-- ============================================================================
-- Indexes for Reviews Table
-- ============================================================================

-- Index on product_id for foreign key lookups
CREATE INDEX IF NOT EXISTS idx_reviews_product_id ON reviews(product_id);

-- Index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_reviews_deleted_at ON reviews(deleted_at);

-- Composite index for reviews list query
-- Covers WHERE product_id = X AND deleted_at IS NULL ORDER BY created_at DESC
CREATE INDEX IF NOT EXISTS idx_reviews_product_deleted_created
ON reviews(product_id, deleted_at, created_at DESC)
WHERE deleted_at IS NULL;
