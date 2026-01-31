.PHONY: help build test test-integration lint docker-build docker-up docker-down migrate-up migrate-down tidy clean swagger dev-infra dev-db-setup dev dev-down

help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  make dev              - Start API locally (runs dev-infra, dev-db-setup, then API)"
	@echo "  make dev-infra        - Start infrastructure services only (postgres, redis, nats)"
	@echo "  make dev-db-setup     - Setup database and run migrations"
	@echo "  make dev-down         - Stop infrastructure services"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build            - Build the API and notifier services"
	@echo "  make test             - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make lint             - Run golangci-lint"
	@echo "  make swagger          - Generate Swagger documentation"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build     - Build Docker images"
	@echo "  make docker-up        - Start all services with docker-compose"
	@echo "  make docker-down      - Stop all services"
	@echo ""
	@echo "Database:"
	@echo "  make migrate-up       - Run database migrations up"
	@echo "  make migrate-down     - Run database migrations down"
	@echo ""
	@echo "Misc:"
	@echo "  make tidy             - Run go mod tidy"
	@echo "  make clean            - Clean build artifacts"

build:
	@echo "Building API service..."
	@go build -o bin/api cmd/api/main.go
	@echo "Building notifier service..."
	@go build -o bin/notifier cmd/notifier/main.go
	@echo "Build complete!"

test:
	@echo "Running unit tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Tests complete! Coverage report: coverage.html"

test-integration:
	@echo "Running integration tests..."
	@go test -v -race -tags=integration ./tests/integration/...

lint:
	@echo "Running linter..."
	@golangci-lint run --config .golangci.yml

swagger:
	@echo "Generating Swagger documentation..."
	@swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal
	@echo "Swagger docs generated at docs/"

docker-build:
	@echo "Building Docker images..."
	@docker-compose build

docker-up:
	@echo "Starting services with docker-compose..."
	@docker-compose up -d
	@echo "Services started! API available at http://localhost:8080"
	@echo "Run 'docker-compose logs -f' to view logs"

docker-down:
	@echo "Stopping services..."
	@docker-compose down

migrate-up:
	@echo "Running migrations..."
	@docker-compose exec postgres psql -U postgres -d product_reviews -c "SELECT 1" > /dev/null 2>&1 || (echo "Database not ready" && exit 1)
	@DB_PORT=$$(grep DB_PORT .env | head -1 | cut -d'=' -f2); \
	migrate -path migrations -database "postgresql://postgres:postgres@localhost:$$DB_PORT/product_reviews?sslmode=disable" up
	@echo "Migrations complete!"

migrate-down:
	@echo "Rolling back migrations..."
	@DB_PORT=$$(grep DB_PORT .env | head -1 | cut -d'=' -f2); \
	migrate -path migrations -database "postgresql://postgres:postgres@localhost:$$DB_PORT/product_reviews?sslmode=disable" down
	@echo "Rollback complete!"

tidy:
	@echo "Running go mod tidy..."
	@go mod tidy
	@go mod download

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "Clean complete!"

dev-infra:
	@echo "Starting infrastructure services (postgres, redis, nats)..."
	@if [ ! -f .env ]; then \
		echo "Creating .env from .env.example..."; \
		cp .env.example .env; \
	fi
	@docker-compose up -d postgres redis nats
	@echo "Waiting for services to be healthy..."
	@sleep 3
	@docker-compose ps
	@echo ""
	@echo "Infrastructure services started!"
	@echo "  PostgreSQL: localhost:$$(grep DB_PORT_EXTERNAL .env | cut -d'=' -f2)"
	@echo "  Redis:      localhost:$$(grep REDIS_PORT_EXTERNAL .env | cut -d'=' -f2)"
	@echo "  NATS:       localhost:$$(grep NATS_PORT_EXTERNAL .env | cut -d'=' -f2)"

dev-db-setup:
	@echo "Setting up database..."
	@DB_PORT=$$(grep DB_PORT_EXTERNAL .env | cut -d'=' -f2); \
	docker-compose exec postgres psql -U postgres -c "SELECT 1 FROM pg_database WHERE datname = 'product_reviews'" | grep -q 1 || \
	(echo "Creating database product_reviews..." && docker-compose exec postgres psql -U postgres -c "CREATE DATABASE product_reviews;")
	@echo "Running migrations..."
	@docker-compose exec postgres psql -U postgres -d product_reviews -f /dev/stdin < migrations/000001_create_products_table.up.sql > /dev/null 2>&1 || echo "Migration 1 already applied"
	@docker-compose exec postgres psql -U postgres -d product_reviews -f /dev/stdin < migrations/000002_create_reviews_table.up.sql > /dev/null 2>&1 || echo "Migration 2 already applied"
	@echo "Database setup complete!"

dev: dev-infra dev-db-setup build
	@echo ""
	@echo "Starting API server in development mode..."
	@echo "API will be available at http://localhost:8080"
	@echo "Swagger docs at http://localhost:8080/swagger/index.html"
	@echo ""
	@echo "Press Ctrl+C to stop"
	@echo ""
	@./bin/api

dev-down:
	@echo "Stopping infrastructure services..."
	@docker-compose stop postgres redis nats
	@echo "Infrastructure services stopped!"
