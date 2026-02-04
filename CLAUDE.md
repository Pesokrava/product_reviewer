# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## LLM Navigation and Assistance

This document is specifically structured to facilitate navigation and comprehension by large language models (LLMs) and their associated code analysis tools, such as `tldr mcp`. It aims to provide a concise yet comprehensive overview of the codebase, essential development commands, architectural patterns, and critical implementation details, enabling efficient understanding and interaction.



## Essential Development Commands

### Quick Start (Development Server)
```bash
make install-dev-tools        # One-time: Install Air (hot reload) and Delve (debugger)
                             # Optional but recommended for development

make dev                      # Start development server
                             # - Starts infrastructure (postgres, redis, nats)
                             # - Creates database and runs migrations
                             # - If Air+Delve installed: hot reload + debug on :2345
                             # - If not installed: runs normally (fallback mode)
                             # API at http://localhost:8080
                             # Swagger at http://localhost:8080/docs

make dev-down                 # Stop infrastructure services
```

### Step-by-Step Development
```bash
make dev-infra               # Start infrastructure services only
make dev-db-setup            # Create database and run migrations
make build && ./bin/api      # Build and run API manually (no hot reload)
```

Note: `make dev` automatically handles infrastructure setup and chooses the best mode (hot reload if available, fallback otherwise).

### Building
```bash
make build                    # Build API, notifier, and rating-worker services to bin/
go build -o bin/api cmd/api/main.go
go build -o bin/notifier cmd/notifier/main.go
go build -o bin/rating-worker cmd/rating-worker/main.go
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
# Access at http://localhost:8080/docs when API is running
```

### Docker Operations
```bash
make docker-up               # Start all services (postgres, redis, nats, api, notifier, rating-worker)
make docker-down             # Stop all services
make docker-build            # Build Docker images
docker-compose logs -f api   # View API logs
docker-compose logs -f notifier  # View notifier logs
docker-compose logs -f rating-worker  # View rating-worker logs
```

### Database Migrations
```bash
make migrate-up              # Apply all pending migrations
make migrate-down            # Rollback last migration
# Migrations are in migrations/ directory
# Requires docker services to be running
```

### Port Configuration

External ports are configurable via `.env` to avoid conflicts:
- PostgreSQL: `DB_PORT_EXTERNAL` (default: 5434)
- Redis: `REDIS_PORT_EXTERNAL` (default: 6379)
- NATS: `NATS_PORT_EXTERNAL` (default: 4222)

The application connects using `DB_PORT`, `REDIS_PORT`, etc. in `.env`.

### Hot Reload & Debugging (Optional)

The development server can use **Air** for hot reload and **Delve** for debugging when available:

**Setup** (optional but recommended):
```bash
make install-dev-tools        # Install Air and Delve
```

**Features** (when tools are installed):
- Automatic rebuild on `.go` file changes
- Debug server on port 2345 (Delve)
- No need to restart after code changes
- Compatible with any Delve-compatible debugger

**Graceful Degradation**:
- `make dev` automatically detects if Air/Delve are installed
- If present: runs with hot reload + debug server
- If not present: runs normally without these features
- No need to change your workflow or commands

**Workflow** (with tools installed):
1. Start: `make dev`
2. Connect your debugger to `localhost:2345`
3. Set breakpoints and debug as usual
4. Code changes auto-rebuild while debugging

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
   - Thread-safe: Concurrency handled by database (MVCC, optimistic locking)

3. **Repository Layer** (`internal/repository/`):
   - Implements domain repository interfaces
   - `postgres/`: Database access using sqlx
   - `cache/`: Redis caching with TTL management
   - Handles: CRUD operations, transactions, cache invalidation

4. **Delivery Layer** (`internal/delivery/`):
   - HTTP handlers (`http/handler/`): Product and Review endpoints
   - Middleware (`http/middleware/`): Logger, Recovery, Timeout (CORS via go-chi/cors)
   - Event system (`events/`): NATS JetStream publisher and stream configuration
   - Request/response helpers for consistent API formatting

5. **Worker Layer** (`internal/worker/`):
   - Rating calculator: Computes average ratings from review data
   - Rating worker: Consumes NATS events and updates product ratings asynchronously
   - Stream configuration: Wraps JetStream setup for worker usage
   - Handles: Debouncing, exponential backoff, idempotent recalculation

### Critical Implementation Details

#### Concurrency-Safe Rating Calculation

The system uses a **two-layer approach** to ensure average_rating is eventually correct:

1. **Layer 1: Cache Invalidation (Immediate Consistency)**:
   - On any review write operation, invalidate ALL cache for that product
   - Pattern: `s.cache.InvalidateAllProductCache(ctx, productID)`
   - Clears both rating cache and all paginated review lists
   - Redis operations (Del, SMembers, Unlink) are atomic
   - Cache invalidation is **non-fatal** - write operations succeed even if Redis is down

2. **Layer 2: Asynchronous Rating Worker (Source of Truth)**:
   - Review service publishes events to NATS JetStream (`reviews.events` subject)
   - JetStream provides persistence (survives restarts) and automatic redelivery
   - Rating worker (`cmd/rating-worker/main.go` + `internal/worker/`) subscribes to durable consumer
   - Worker debounces updates (1-second window) to batch multiple events for the same product
   - Exponential backoff retry: 3 attempts total (immediate, then 1s wait, then 2s wait)
   - After 3 failed attempts, message is discarded (next review event will recalculate)
   - Worker executes SQL: `UPDATE products SET average_rating = ..., version = version + 1 WHERE id = ?`
   - PostgreSQL MVCC handles concurrent access safely without application-level locks
   - Rating calculation is idempotent and self-correcting (full recalculation from DB state)
   - Concurrency limited to 10 simultaneous calculations to prevent DB overload

**IMPORTANT**: When adding new review operations, always:
- Call `InvalidateAllProductCache` after DB write (log warning if it fails, don't fail the operation)
- Publish event to NATS for worker to calculate rating asynchronously
- No application-level locking needed - database handles concurrency
- Accept temporary rating staleness in exchange for availability (eventual consistency)

#### Caching Strategy

Cache-aside pattern with aggressive invalidation:

```go
// Product rating cache
Key: "product:{id}:rating"
TTL: 5 minutes (CACHE_TTL_PRODUCT_RATING)

// Reviews list cache
Key: "product:{id}:reviews:limit:{limit}:offset:{offset}"
TTL: 2 minutes (CACHE_TTL_REVIEWS_LIST)
```

**Read flow**:
1. Check cache first
2. On miss: query DB, store in cache, return
3. On hit: return cached value

**Write flow**:
1. Update database
2. Invalidate ALL related cache keys (uses SET-based tracking for paginated lists)
3. Publish event to NATS
4. Return response

Cache invalidation happens in `internal/repository/cache/redis.go`:
- `InvalidateProductRating()`: Clear single product rating
- `InvalidateReviewsList()`: Clear all review pages using SET-based tracking (SMembers + Unlink)
- `InvalidateAllProductCache()`: Clear rating + all review pages atomically

#### Event System

NATS JetStream for durable, reliable event delivery:

- **Publisher**: `internal/delivery/events/publisher.go` (JetStream publisher with ack)
- **Stream Config**: `internal/delivery/events/stream.go` (stream and consumer setup)
- **Consumer**: Rating worker (`cmd/rating-worker/main.go`) uses durable pull consumer
- **Subject**: `reviews.events`
- **Event Types**: `review.created`, `review.updated`, `review.deleted`

**JetStream Features:**
- **Persistence**: Messages survive worker restarts (file storage)
- **Redelivery**: JetStream redelivers unacked messages with backoff (1s, 2s, 4s)
- **Durability**: Pull consumer with durable name `rating-worker`
- **Acknowledgment**: Explicit ack required (AckExplicitPolicy)
- **MaxDeliver**: 3 JetStream delivery attempts, then discard
- **Worker Retries**: Each delivery attempt has internal worker retries (immediate, 1s, 2s)

Events are published with acknowledgment in `internal/usecase/review/service.go`:
```go
s.publishEvent(ctx, "review.created", review)
```

The rating-worker service consumes events, processes rating calculations, and acknowledges successful processing. The notifier service (`cmd/notifier/main.go`) demonstrates an alternative consumption pattern for notifications.

**Why no Dead Letter Queue?**
Rating calculation is idempotent and based on database state (full recalculation). If an event fails after 3 attempts, it's discarded because the next review event will trigger a full recalculation that corrects any missed updates.

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
- Good: `// Async worker handles calculation to avoid blocking write operations and ensure retry capability`
- Bad: `// Loop through reviews and calculate average`
- Good: `// Fire-and-forget pattern to avoid blocking API response while ensuring event delivery`
- Bad: `// Publish event in goroutine`

If you find yourself writing a comment that describes HOW the code works, consider refactoring the code to be more self-documenting instead (better variable names, extracted functions, etc.).

### Go Type Standards

**Always use `any` instead of `interface{}`**:
- `any` is the modern Go 1.18+ alias for `interface{}`
- More readable and idiomatic in modern Go code
- Good: `func Process(data any) error`
- Bad: `func Process(data interface{}) error`
- Apply this consistently across function signatures, type assertions, and type definitions

### API Documentation

**Every API change MUST be reflected in Swagger/OpenAPI documentation**:
- When adding a new endpoint: Add Swagger annotations to the handler function
- When modifying endpoint behavior: Update the annotations (parameters, responses, descriptions)
- When changing request/response models: Update the struct field tags and annotations
- After any API changes: Run `make swagger` to regenerate docs
- The Swagger UI at http://localhost:8080/docs is the source of truth for API consumers

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
- Access UI: http://localhost:8080/docs
- **Important**: See "API Documentation" rule in Code Standards section - all API changes must update Swagger docs

## Common Gotchas

1. **Don't manually calculate average_rating** - The rating-worker service does this asynchronously via NATS events
2. **Always invalidate cache after write operations** - Stale cache causes inconsistencies
3. **Database handles concurrency** - No service-level mutexes needed; PostgreSQL MVCC + optimistic locking handle concurrent access safely
4. **Product updates use optimistic locking** - Check `version` field to prevent conflicts
5. **Soft deletes** - Use `deleted_at` timestamp, don't physically delete records
6. **Event publishing is async** - Don't rely on events for critical business logic
7. **Context propagation** - Always pass context through service layers for cancellation
8. **UUID validation** - Use `request.GetUUIDParam()` helper to parse and validate UUIDs
9. **Pagination** - Default limit is 20, max is 100 (enforced in handlers)
10. **Migrations require running services** - Start docker-compose before running migrations
11. **Product version conflicts** - Product.version increments when rating-worker updates average_rating. Product updates can fail with ErrConflict due to concurrent rating calculation, not just concurrent product updates. This is by design - the product DID change (rating changed).

## Debugging

### View Cache Contents
```bash
docker-compose exec redis redis-cli
> KEYS product:*
> GET product:{uuid}:rating
> TTL product:{uuid}:rating
> GET product:{uuid}:reviews:limit:20:offset:0
> SMEMBERS product:{uuid}:cache_keys  # List all cached review pages for a product
```

### View NATS Events
```bash
docker-compose logs -f notifier
# Shows all published review events in real-time

docker-compose logs -f rating-worker
# Shows rating calculation processing and retries
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
