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

-- Create index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_products_deleted_at ON products(deleted_at);

-- Create index on name for search queries
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
