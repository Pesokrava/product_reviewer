-- Composite index for products list query
-- Covers WHERE deleted_at IS NULL ORDER BY created_at DESC queries efficiently
-- Note: Using CREATE INDEX (not CONCURRENTLY) since this runs on startup before traffic
CREATE INDEX IF NOT EXISTS idx_products_deleted_at_created_at
ON products(deleted_at, created_at DESC)
WHERE deleted_at IS NULL;

-- Composite index for reviews list query
-- Covers WHERE product_id = X AND deleted_at IS NULL ORDER BY created_at DESC queries
CREATE INDEX IF NOT EXISTS idx_reviews_product_deleted_created
ON reviews(product_id, deleted_at, created_at DESC)
WHERE deleted_at IS NULL;

-- Remove old less-efficient index that is now redundant
DROP INDEX IF EXISTS idx_reviews_product_id_deleted_at;
