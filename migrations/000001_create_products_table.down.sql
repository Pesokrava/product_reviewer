-- Drop indexes
DROP INDEX IF EXISTS idx_products_name;
DROP INDEX IF EXISTS idx_products_deleted_at;

-- Drop products table
DROP TABLE IF EXISTS products;
