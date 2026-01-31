.PHONY: help build test test-integration lint docker-build docker-up docker-down migrate-up migrate-down tidy clean swagger

help:
	@echo "Available commands:"
	@echo "  make build            - Build the API and notifier services"
	@echo "  make test             - Run unit tests"
	@echo "  make test-integration - Run integration tests"
	@echo "  make lint             - Run golangci-lint"
	@echo "  make swagger          - Generate Swagger documentation"
	@echo "  make docker-build     - Build Docker images"
	@echo "  make docker-up        - Start all services with docker-compose"
	@echo "  make docker-down      - Stop all services"
	@echo "  make migrate-up       - Run database migrations up"
	@echo "  make migrate-down     - Run database migrations down"
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
	@migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/product_reviews?sslmode=disable" up
	@echo "Migrations complete!"

migrate-down:
	@echo "Rolling back migrations..."
	@migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/product_reviews?sslmode=disable" down
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
