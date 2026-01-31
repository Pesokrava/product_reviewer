-- Drop trigger
DROP TRIGGER IF EXISTS trigger_update_product_rating ON reviews;

-- Drop function
DROP FUNCTION IF EXISTS update_product_average_rating();

-- Drop indexes
DROP INDEX IF EXISTS idx_reviews_product_id_deleted_at;
DROP INDEX IF EXISTS idx_reviews_deleted_at;
DROP INDEX IF EXISTS idx_reviews_product_id;

-- Drop reviews table
DROP TABLE IF EXISTS reviews;
