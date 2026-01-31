# Product Reviews System

A production-ready product reviews system built in Go with RESTful APIs, Redis caching, PostgreSQL database, and NATS event notifications.

## Features

- RESTful API for products and reviews
- Automatic rating calculation with database triggers (concurrency-safe)
- Redis caching with TTL and invalidation
- Event notifications via NATS pub/sub
- Graceful shutdown and health checks
- Docker Compose setup for easy deployment
- Clean architecture with separation of concerns
- Comprehensive error handling and validation

## Architecture

The system follows Clean Architecture with four distinct layers:

1. **Domain Layer** (`internal/domain/`): Core entities and repository interfaces
2. **Use Case Layer** (`internal/usecase/`): Business logic, caching, and event publishing
3. **Repository Layer** (`internal/repository/`): Data access implementations (PostgreSQL, Redis)
4. **Delivery Layer** (`internal/delivery/`): HTTP handlers, middleware, and event consumers

### Technology Stack

- **Language**: Go 1.25+
- **Web Framework**: Chi (lightweight, idiomatic router)
- **Database**: PostgreSQL 15+ (ACID compliance, triggers)
- **Cache**: Redis 7+ (fast, TTL support)
- **Message Broker**: NATS (lightweight, cloud-native)
- **Key Libraries**: sqlx, validator, zerolog, viper

### Concurrency Safety

The system uses a three-layer approach for concurrency safety:

1. **Database Level (Primary)**: PostgreSQL trigger automatically updates `average_rating` atomically within transaction context
2. **Application Level (Secondary)**: Mutex in service layer for critical sections
3. **Cache Level**: Redis atomic operations with invalidation on writes

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.25+ (for local development)
- Make (optional, for convenience commands)

### Setup with Docker

1. Clone the repository
2. Start all services:

```bash
make docker-up
```

This starts:
- PostgreSQL (port 5432)
- Redis (port 6379)
- NATS (port 4222)
- API service (port 8080)
- Notifier service (logs events)

3. Run database migrations:

```bash
make migrate-up
```

4. Verify the API is running:

```bash
curl http://localhost:8080/health
```

### Local Development

**Quick Start:**

```bash
make dev
```

This single command:
- Creates `.env` from `.env.example` if needed
- Starts infrastructure services (PostgreSQL, Redis, NATS)
- Creates database and runs migrations
- Builds and runs the API locally

**Step-by-Step (if you prefer manual control):**

1. Start infrastructure services:

```bash
make dev-infra
```

2. Setup database and run migrations:

```bash
make dev-db-setup
```

3. Build and run the API:

```bash
make build
./bin/api
```

4. Run the notifier service (in another terminal):

```bash
./bin/notifier
```

**Stop infrastructure services:**

```bash
make dev-down
```

## API Documentation

### Base URL

```
http://localhost:8080/api/v1
```

### Products API

#### Create Product

```bash
POST /api/v1/products

curl -X POST http://localhost:8080/api/v1/products \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Laptop Pro 2024",
    "description": "High-performance laptop for professionals",
    "price": 1299.99
  }'
```

Response:
```json
{
  "success": true,
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "Laptop Pro 2024",
    "description": "High-performance laptop for professionals",
    "price": 1299.99,
    "average_rating": 0,
    "version": 1,
    "created_at": "2026-01-30T10:00:00Z",
    "updated_at": "2026-01-30T10:00:00Z"
  }
}
```

#### Get Product

```bash
GET /api/v1/products/:id

curl http://localhost:8080/api/v1/products/123e4567-e89b-12d3-a456-426614174000
```

Response:
```json
{
  "success": true,
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "name": "Laptop Pro 2024",
    "description": "High-performance laptop for professionals",
    "price": 1299.99,
    "average_rating": 4.5,
    "version": 1,
    "created_at": "2026-01-30T10:00:00Z",
    "updated_at": "2026-01-30T10:00:00Z"
  }
}
```

Note: Product info does NOT include reviews array. Use the separate reviews endpoint.

#### List Products

```bash
GET /api/v1/products?limit=20&offset=0

curl "http://localhost:8080/api/v1/products?limit=10&offset=0"
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "name": "Laptop Pro 2024",
      "price": 1299.99,
      "average_rating": 4.5,
      ...
    }
  ],
  "pagination": {
    "total": 50,
    "limit": 10,
    "offset": 0
  }
}
```

#### Update Product

```bash
PUT /api/v1/products/:id

curl -X PUT http://localhost:8080/api/v1/products/123e4567-e89b-12d3-a456-426614174000 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Laptop Pro 2024 Updated",
    "description": "Updated description",
    "price": 1199.99
  }'
```

#### Delete Product

```bash
DELETE /api/v1/products/:id

curl -X DELETE http://localhost:8080/api/v1/products/123e4567-e89b-12d3-a456-426614174000
```

#### Get Product Reviews

```bash
GET /api/v1/products/:id/reviews?limit=20&offset=0

curl "http://localhost:8080/api/v1/products/123e4567-e89b-12d3-a456-426614174000/reviews?limit=10&offset=0"
```

Response:
```json
{
  "success": true,
  "data": [
    {
      "id": "456e7890-e89b-12d3-a456-426614174001",
      "product_id": "123e4567-e89b-12d3-a456-426614174000",
      "first_name": "John",
      "last_name": "Doe",
      "review_text": "Great product! Highly recommended.",
      "rating": 5,
      "created_at": "2026-01-30T11:00:00Z",
      "updated_at": "2026-01-30T11:00:00Z"
    }
  ],
  "pagination": {
    "total": 25,
    "limit": 10,
    "offset": 0
  }
}
```

### Reviews API

#### Create Review

```bash
POST /api/v1/reviews

curl -X POST http://localhost:8080/api/v1/reviews \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": "123e4567-e89b-12d3-a456-426614174000",
    "first_name": "John",
    "last_name": "Doe",
    "review_text": "Great product! Highly recommended.",
    "rating": 5
  }'
```

Rating must be between 1 and 5. The product's average rating is automatically updated via database trigger.

Response:
```json
{
  "success": true,
  "data": {
    "id": "456e7890-e89b-12d3-a456-426614174001",
    "product_id": "123e4567-e89b-12d3-a456-426614174000",
    "first_name": "John",
    "last_name": "Doe",
    "review_text": "Great product! Highly recommended.",
    "rating": 5,
    "created_at": "2026-01-30T11:00:00Z",
    "updated_at": "2026-01-30T11:00:00Z"
  }
}
```

#### Update Review

```bash
PUT /api/v1/reviews/:id

curl -X PUT http://localhost:8080/api/v1/reviews/456e7890-e89b-12d3-a456-426614174001 \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "John",
    "last_name": "Doe",
    "review_text": "Updated review text",
    "rating": 4
  }'
```

#### Delete Review

```bash
DELETE /api/v1/reviews/:id

curl -X DELETE http://localhost:8080/api/v1/reviews/456e7890-e89b-12d3-a456-426614174001
```

### Health Check

```bash
GET /health

curl http://localhost:8080/health
```

## Caching Strategy

The system uses a cache-aside pattern with Redis:

1. **Product Rating Cache**:
   - Key: `product:{id}:rating`
   - TTL: 5 minutes
   - Invalidated on: Any review create/update/delete

2. **Product Reviews List Cache**:
   - Key: `product:{id}:reviews:page:{page}`
   - TTL: 2 minutes
   - Invalidated on: Review changes for that product

### Cache Flow

- **Read**: Check cache → if miss: query DB → store in cache → return
- **Write**: Update DB → invalidate related cache keys → publish event

## Event Notifications

The system publishes events to NATS on review operations:

- Subject: `reviews.events`
- Event types: `review.created`, `review.updated`, `review.deleted`

The notifier service subscribes to these events and logs them. You can extend this to send emails, webhooks, etc.

### Event Payload Example

```json
{
  "event_type": "review.created",
  "timestamp": "2026-01-30T11:00:00Z",
  "product_id": "123e4567-e89b-12d3-a456-426614174000",
  "review": {
    "id": "456e7890-e89b-12d3-a456-426614174001",
    "first_name": "John",
    "last_name": "Doe",
    "rating": 5,
    "review_text": "Great product!"
  }
}
```

## Testing

### Run Unit Tests

```bash
make test
```

### Run Integration Tests

```bash
make test-integration
```

### Run Linter

```bash
make lint
```

### Manual Testing

1. Create a product
2. Create multiple reviews for the product
3. Verify the average_rating updates automatically
4. Check the notifier service logs for events
5. Query reviews and verify caching (check Redis with `redis-cli KEYS *`)

## Database Schema

### Products Table

```sql
CREATE TABLE products (
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
```

### Reviews Table

```sql
CREATE TABLE reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    review_text TEXT NOT NULL,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP
);
```

### Database Trigger

A PostgreSQL trigger automatically updates the product's average_rating whenever a review is created, updated, or deleted. This ensures atomic, concurrency-safe rating calculations.

## Makefile Commands

**Development:**
- `make dev` - Start API locally (runs dev-infra, dev-db-setup, then API)
- `make dev-infra` - Start infrastructure services only (postgres, redis, nats)
- `make dev-db-setup` - Setup database and run migrations
- `make dev-down` - Stop infrastructure services

**Build & Test:**
- `make build` - Build API and notifier services
- `make test` - Run unit tests
- `make test-integration` - Run integration tests
- `make lint` - Run linter
- `make swagger` - Generate Swagger documentation

**Docker:**
- `make docker-build` - Build Docker images
- `make docker-up` - Start all services
- `make docker-down` - Stop all services

**Database:**
- `make migrate-up` - Run database migrations
- `make migrate-down` - Rollback migrations

**Misc:**
- `make tidy` - Run go mod tidy
- `make clean` - Clean build artifacts

## Project Structure

```
product-reviews/
├── cmd/
│   ├── api/main.go              # Main API service
│   └── notifier/main.go         # Event consumer service
├── internal/
│   ├── domain/                  # Core entities and interfaces
│   ├── usecase/                 # Business logic
│   ├── repository/              # Data access (PostgreSQL, Redis)
│   ├── delivery/                # HTTP handlers, event system
│   ├── config/                  # Configuration management
│   └── pkg/                     # Shared packages (logger, database, cache)
├── migrations/                  # SQL migrations
├── docker-compose.yml           # All services
├── Dockerfile                   # Multi-stage build
├── Makefile                     # Common commands
└── README.md                    # This file
```

## Environment Variables

See `.env.example` for all available configuration options.

Key variables:
- `ENV` - Environment (development/production)
- `SERVER_PORT` - HTTP server port (default: 8080)
- `DB_*` - PostgreSQL connection settings
- `REDIS_*` - Redis connection settings
- `NATS_URL` - NATS connection URL
- `CACHE_TTL_*` - Cache TTL settings

## Troubleshooting

### Database connection issues

```bash
docker-compose logs postgres
```

### Redis connection issues

```bash
docker-compose logs redis
```

### Check Redis cache

```bash
docker-compose exec redis redis-cli
> KEYS *
> GET product:{id}:rating
```

### View NATS events

Check the notifier service logs:

```bash
docker-compose logs -f notifier
```

## License

MIT
