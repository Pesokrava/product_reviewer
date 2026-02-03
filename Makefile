.PHONY: help build test test-integration lint docker-build docker-up docker-down migrate-up migrate-down tidy clean swagger dev-infra dev-db-setup dev dev-down dev-clean install-dev-tools

help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  make dev              - Start API (with hot reload + debug if tools installed)"
	@echo "  make dev-infra        - Start infrastructure services only (postgres, redis, nats)"
	@echo "  make dev-db-setup     - Setup database and run migrations"
	@echo "  make dev-down         - Stop infrastructure services"
	@echo "  make dev-clean        - Stop services and remove all volumes (fresh start)"
	@echo "  make install-dev-tools - Install Air and Delve for hot reload and debugging"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build            - Build API, notifier, and rating-worker services"
	@echo "  make test             - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make lint             - Run golangci-lint"
	@echo "  make swagger          - Generate Swagger documentation"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build     - Build Docker images"
	@echo "  make docker-up        - Start all services (api, notifier, rating-worker)"
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
	@echo "Building rating-worker service..."
	@go build -o bin/rating-worker cmd/rating-worker/main.go
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
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if docker-compose exec -T postgres pg_isready -U postgres > /dev/null 2>&1; then \
			echo "PostgreSQL is ready!"; \
			break; \
		fi; \
		echo "Waiting for PostgreSQL... ($$timeout seconds remaining)"; \
		sleep 2; \
		timeout=$$((timeout - 2)); \
	done; \
	if [ $$timeout -le 0 ]; then \
		echo "ERROR: PostgreSQL failed to start within 60 seconds"; \
		exit 1; \
	fi
	@docker-compose ps
	@echo ""
	@echo "Infrastructure services started!"
	@echo "  PostgreSQL: localhost:$$(grep DB_PORT_EXTERNAL .env | cut -d'=' -f2)"
	@echo "  Redis:      localhost:$$(grep REDIS_PORT_EXTERNAL .env | cut -d'=' -f2)"
	@echo "  NATS:       localhost:$$(grep NATS_PORT_EXTERNAL .env | cut -d'=' -f2)"

dev-db-setup:
	@echo "Setting up database..."
	@echo "Checking if database exists..."
	@if docker-compose exec -T postgres psql -U postgres -lqt | cut -d \| -f 1 | grep -qw product_reviews; then \
		echo "Database 'product_reviews' already exists"; \
	else \
		echo "Creating database 'product_reviews'..."; \
		docker-compose exec -T postgres psql -U postgres -c "CREATE DATABASE product_reviews;"; \
		echo "Database created successfully!"; \
	fi
	@echo "Running migrations..."
	@docker-compose exec -T postgres psql -U postgres -d product_reviews -f /dev/stdin < migrations/000001_create_schema.up.sql > /dev/null 2>&1 || echo "Migration already applied or failed"
	@echo "Database setup complete!"
	@echo "Verifying database..."
	@docker-compose exec -T postgres psql -U postgres -d product_reviews -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public';"

dev: dev-infra dev-db-setup
	@echo ""
	@if command -v air > /dev/null 2>&1 && command -v dlv > /dev/null 2>&1; then \
		echo "Starting API server with hot reload and debug server..."; \
		echo ""; \
		echo "  API:        http://localhost:8080"; \
		echo "  Swagger:    http://localhost:8080/docs"; \
		echo "  Debug Port: localhost:2345 (Delve)"; \
		echo ""; \
		echo "Hot reload is enabled - changes will auto-rebuild"; \
		echo "Connect your debugger to localhost:2345"; \
		echo ""; \
		echo "Press Ctrl+C to stop"; \
		echo ""; \
		set -a; . ./.env; set +a; air -c .air.toml; \
	else \
		echo "Air and/or Delve not found. Running without hot reload."; \
		echo "For hot reload + debugging, install tools with: make install-dev-tools"; \
		echo ""; \
		echo "Starting API server..."; \
		echo "API will be available at http://localhost:8080"; \
		echo "Swagger docs at http://localhost:8080/docs"; \
		echo ""; \
		echo "Press Ctrl+C to stop"; \
		echo ""; \
		$(MAKE) build; \
		set -a; . ./.env; set +a; ./bin/api; \
	fi

dev-down:
	@echo "Stopping infrastructure services..."
	@docker-compose stop postgres redis nats
	@echo "Infrastructure services stopped!"

dev-clean:
	@echo "Cleaning up all infrastructure services and volumes..."
	@docker-compose down -v
	@echo "All services stopped and volumes removed!"
	@echo "Run 'make dev' to start fresh"

install-dev-tools:
	@echo "Installing development tools..."
	@echo ""
	@echo "Installing Air (hot reload)..."
	@go install github.com/air-verse/air@latest
	@echo "Installing Delve (debugger)..."
	@go install github.com/go-delve/delve/cmd/dlv@latest
	@echo ""
	@echo "Development tools installed successfully!"
	@echo ""
	@echo "Verify installation:"
	@which air && air -v || echo "  Air: NOT FOUND"
	@which dlv && dlv version || echo "  Delve: NOT FOUND"
