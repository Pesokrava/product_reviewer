-- Drop tables in reverse order (reviews first due to foreign key)
DROP TABLE IF EXISTS reviews CASCADE;
DROP TABLE IF EXISTS products CASCADE;

-- Indexes are automatically dropped with their tables
