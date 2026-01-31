-- Remove performance indexes
DROP INDEX IF EXISTS idx_products_deleted_at_created_at;
DROP INDEX IF EXISTS idx_reviews_product_deleted_created;

-- Restore old index for backwards compatibility
CREATE INDEX IF NOT EXISTS idx_reviews_product_id_deleted_at ON reviews(product_id, deleted_at);
