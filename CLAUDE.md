# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Development Commands

### Building
```bash
make build                    # Build both API and notifier services to bin/
go build -o bin/api cmd/api/main.go
go build -o bin/notifier cmd/notifier/main.go
```

### Testing
```bash
make test                     # Run all unit tests with race detector and coverage
go test -v ./internal/usecase/product/...           # Test specific package
go test -v -run TestService_Create_Success ./...    # Run single test
make test-integration         # Run integration tests (requires docker services)
```

### Linting
```bash
make lint                     # Run golangci-lint with .golangci.yml config
```

### API Documentation
```bash
make swagger                  # Generate Swagger docs from annotations
# Docs generated to docs/ directory
# Access at http://localhost:8080/swagger/index.html when API is running
```

### Docker Operations
```bash
make docker-up               # Start all services (postgres, redis, nats, api, notifier)
make docker-down             # Stop all services
make docker-build            # Build Docker images
docker-compose logs -f api   # View API logs
docker-compose logs -f notifier  # View notifier logs
```

### Database Migrations
```bash
make migrate-up              # Apply all pending migrations
make migrate-down            # Rollback last migration
# Migrations are in migrations/ directory
# Requires docker services to be running
```

### Local Development Workflow
```bash
cp .env.example .env                    # Setup environment variables
docker-compose up -d postgres redis nats  # Start infrastructure only
make migrate-up                         # Apply migrations
go run cmd/api/main.go                  # Run API locally
go run cmd/notifier/main.go            # Run notifier in separate terminal
```

## Architecture Overview

### Clean Architecture Layers

This codebase follows **Clean Architecture** with strict dependency rules:

1. **Domain Layer** (`internal/domain/`):
   - Core entities: `Product`, `Review`
   - Repository interfaces: `ProductRepository`, `ReviewRepository`
   - Domain errors: `ErrNotFound`, `ErrInvalidInput`, etc.
   - **Zero external dependencies** - only standard library and basic packages

2. **Use Case Layer** (`internal/usecase/`):
   - Business logic services: `product.Service`, `review.Service`
   - Depends on: domain interfaces, validator, logger
   - Implements: input validation, caching coordination, event publishing
   - Thread-safe with sync.RWMutex for concurrent operations

3. **Repository Layer** (`internal/repository/`):
   - Implements domain repository interfaces
   - `postgres/`: Database access using sqlx
   - `cache/`: Redis caching with TTL management
   - Handles: CRUD operations, transactions, cache invalidation

4. **Delivery Layer** (`internal/delivery/`):
   - HTTP handlers (`http/handler/`): Product and Review endpoints
   - Middleware (`http/middleware/`): Logger, Recovery, CORS
   - Event system (`events/`): NATS publisher/consumer
   - Request/response helpers for consistent API formatting

### Critical Implementation Details

#### Concurrency-Safe Rating Calculation

The system uses a **three-layer approach** to ensure average_rating is always correct:

1. **Database Trigger (Primary Safety Mechanism)**:
   - PostgreSQL trigger in `migrations/000002_create_reviews_table.up.sql`
   - Automatically recalculates `average_rating` on ANY review change (INSERT/UPDATE/DELETE)
   - Runs atomically within transaction context
   - Updates `version` field for optimistic locking
   - This is the **source of truth** - application code relies on this trigger

2. **Application Mutex (Secondary)**:
   - `sync.RWMutex` in `internal/usecase/review/service.go`
   - Protects critical sections during review operations
   - Prevents race conditions in cache invalidation

3. **Cache Invalidation (Consistency)**:
   - On any review write operation, invalidate ALL cache for that product
   - Pattern: `s.cache.InvalidateAllProductCache(ctx, productID)`
   - Clears both rating cache and all paginated review lists

**IMPORTANT**: When adding new review operations, always:
- Wrap in mutex lock/unlock
- Call `InvalidateAllProductCache` after DB write
- Trust the database trigger for rating calculation

#### Caching Strategy

Cache-aside pattern with aggressive invalidation:

```go
// Product rating cache
Key: "product:{id}:rating"
TTL: 5 minutes (CACHE_TTL_PRODUCT_RATING)

// Reviews list cache
Key: "product:{id}:reviews:page:{page}"
TTL: 2 minutes (CACHE_TTL_REVIEWS_LIST)
```

**Read flow**:
1. Check cache first
2. On miss: query DB, store in cache, return
3. On hit: return cached value

**Write flow**:
1. Update database
2. Invalidate ALL related cache keys (use pattern matching for paginated lists)
3. Publish event to NATS
4. Return response

Cache invalidation happens in `internal/repository/cache/redis.go`:
- `InvalidateProductRating()`: Clear single product rating
- `InvalidateAllProductCache()`: Clear rating + all review pages using Redis SCAN

#### Event System

NATS pub/sub for asynchronous notifications:

- **Publisher**: `internal/delivery/events/publisher.go`
- **Consumer**: `internal/delivery/events/consumer.go`
- **Subject**: `reviews.events`
- **Event Types**: `review.created`, `review.updated`, `review.deleted`

Events are published in goroutines (fire-and-forget) in `internal/usecase/review/service.go`:
```go
s.publishEvent(ctx, "review.created", review)
```

The notifier service (`cmd/notifier/main.go`) demonstrates consumption pattern.

### API Design Patterns

#### Important API Behavior

**Product endpoints do NOT return reviews**:
- `GET /api/v1/products/:id` returns product with `average_rating` only
- Use separate endpoint `GET /api/v1/products/:id/reviews` to get reviews
- This design prevents N+1 queries and keeps responses lightweight

#### Request/Response Helpers

- `internal/delivery/http/request/request.go`: Parse JSON, extract UUID params, pagination
- `internal/delivery/http/response/response.go`: Standard response formats
  - `Success()`, `Created()`, `NoContent()` for success responses
  - `Error()` for error responses with proper status codes
  - `Paginated()` for list endpoints with pagination metadata

#### Validation

- Entity validation: go-playground/validator tags in domain structs
- Input validation: Happens in use case services before DB operations
- Example: `validate:"required,min=1,max=255"` on Product.Name

### Configuration Management

- **Package**: `internal/config/config.go`
- **Library**: Viper (loads from environment variables)
- **File**: `.env.example` shows all available options
- **Key configs**:
  - Database connection pool settings
  - Redis connection details
  - NATS URL
  - Cache TTL durations
  - Server timeouts

### Logging

- **Package**: `internal/pkg/logger/logger.go`
- **Library**: zerolog
- **Output**: Console format in development, JSON in production
- **Methods**:
  - `logger.Info()`, `logger.Error()`, etc. for simple messages
  - `logger.WithFields()` for structured logging with context
- **Usage**: Pass logger to services via dependency injection

### Testing Strategy

**Unit Tests**:
- Located next to source files (e.g., `service_test.go`)
- Use mock repositories (see `internal/usecase/product/service_test.go`)
- Test business logic without external dependencies
- Run with: `go test ./internal/...`

**Integration Tests**:
- Located in `tests/integration/`
- Require docker services running
- Test full stack with real database/cache/message broker
- Tagged with `// +build integration`

## Module Path

This project uses: `github.com/Pesokrava/product_reviewer`

When adding new packages, ensure imports use this path.

## Code Standards

### Comments

**Comments should explain WHY, not HOW**:
- The code itself should be self-explanatory about WHAT it does
- Comments should explain the reasoning, context, or business decisions behind the code
- Good: `// Use database trigger instead of application code to ensure atomicity under high concurrency`
- Bad: `// Loop through reviews and calculate average`
- Good: `// Fire-and-forget pattern to avoid blocking API response while ensuring event delivery`
- Bad: `// Publish event in goroutine`

If you find yourself writing a comment that describes HOW the code works, consider refactoring the code to be more self-documenting instead (better variable names, extracted functions, etc.).

### API Documentation

**Every API change MUST be reflected in Swagger/OpenAPI documentation**:
- When adding a new endpoint: Add Swagger annotations to the handler function
- When modifying endpoint behavior: Update the annotations (parameters, responses, descriptions)
- When changing request/response models: Update the struct field tags and annotations
- After any API changes: Run `make swagger` to regenerate docs
- The Swagger UI at http://localhost:8080/swagger/index.html is the source of truth for API consumers

Annotations format:
```go
// @Summary Short description
// @Description Detailed description
// @Tags Category
// @Accept json
// @Produce json
// @Param id path string true "UUID"
// @Success 200 {object} ResponseType
// @Failure 400 {object} map[string]string
// @Router /endpoint/{id} [get]
```

### Code Reviews

**When asked to review code, use "Linus Torvalds" style: harsh but fair, directly to the point**:
- Be brutally honest about problems - don't sugarcoat issues
- Focus on technical merit, not feelings
- Point out bad patterns, inefficiencies, and architectural problems directly
- No diplomatic language - if something is wrong, say it's wrong
- Explain WHY it's wrong and what the correct approach should be
- Be fair: acknowledge good parts, but don't praise unnecessarily
- Get straight to the point - no lengthy explanations unless needed for understanding

## Swagger Documentation

- Annotations are in handler files (`internal/delivery/http/handler/*.go`)
- Main API metadata in `cmd/api/main.go`
- Generate docs: `make swagger`
- Access UI: http://localhost:8080/swagger/index.html
- **Important**: See "API Documentation" rule in Code Standards section - all API changes must update Swagger docs

## Common Gotchas

1. **Don't manually calculate average_rating** - The database trigger does this atomically
2. **Always invalidate cache after write operations** - Stale cache causes inconsistencies
3. **Use mutex in review service** - Prevents race conditions in concurrent scenarios
4. **Product updates use optimistic locking** - Check `version` field to prevent conflicts
5. **Soft deletes** - Use `deleted_at` timestamp, don't physically delete records
6. **Event publishing is async** - Don't rely on events for critical business logic
7. **Context propagation** - Always pass context through service layers for cancellation
8. **UUID validation** - Use `request.GetUUIDParam()` helper to parse and validate UUIDs
9. **Pagination** - Default limit is 20, max is 100 (enforced in handlers)
10. **Migrations require running services** - Start docker-compose before running migrations

## Debugging

### View Cache Contents
```bash
docker-compose exec redis redis-cli
> KEYS product:*
> GET product:{uuid}:rating
> TTL product:{uuid}:rating
```

### View NATS Events
```bash
docker-compose logs -f notifier
# Shows all published review events in real-time
```

### View Database State
```bash
docker-compose exec postgres psql -U postgres -d product_reviews
\dt                           # List tables
SELECT * FROM products;       # View products
SELECT * FROM reviews WHERE product_id = '{uuid}';
```

### Check API Health
```bash
curl http://localhost:8080/health
```
