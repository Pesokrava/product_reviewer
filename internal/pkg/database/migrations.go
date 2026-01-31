package database

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
)

func RunMigrations(db *sqlx.DB) error {
	migrations := []string{
		"migrations/000001_create_products_table.up.sql",
		"migrations/000002_create_reviews_table.up.sql",
		"migrations/000003_add_performance_indexes.up.sql",
	}

	for _, path := range migrations {
		sql, err := os.ReadFile(path)
		if err != nil {
			absPath, _ := filepath.Abs(path)
			return fmt.Errorf("failed to read migration %s (absolute: %s): %w", path, absPath, err)
		}

		if err := executeMigration(db, path, string(sql)); err != nil {
			return fmt.Errorf("migration %s failed: %w", path, err)
		}
	}

	return nil
}

func executeMigration(db *sqlx.DB, name, sql string) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(sql); err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
