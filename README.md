# Product Reviewer Service

A microservices-based product review platform built with Go, PostgreSQL, Redis, and NATS JetStream, following Clean Architecture principles. It supports product management, review submissions, and asynchronous average rating calculation.

## Features

*   RESTful API for Products and Reviews
*   Asynchronous Rating Calculation (NATS JetStream)
*   Redis Caching
*   PostgreSQL Database
*   Event-Driven Architecture

## Prerequisites

Before you begin, ensure you have the following installed:

*   **Go**: Version `1.25.5` or later.
*   **Docker Compose** 
*   **`make`**

## Getting Started

Follow these steps to set up and run the project locally.

### 1. Clone the Repository

```bash
git clone https://github.com/Pesokrava/product_reviewer.git
cd product_reviewer
```

### 2. Environment Configuration

Copy the example environment file and customize it if necessary:

```bash
cp .env.example .env
```

Review the `.env` file to adjust external ports for PostgreSQL, Redis, and NATS if they conflict with other services running on your machine.

### 3. Install Go Development Tools (Required for Database Migrations and Recommended for Hot-Reloading/Debugging)

This step installs essential tools, including the `migrate` command-line tool which is crucial for database migrations, and optionally `Air` for hot-reloading and `Delve` for debugging. The `make dev` command relies on the `migrate` tool to set up the database.


```bash
make install-dev-tools
```

## Running the Application

### Using `make dev` (Recommended for Development)

This command orchestrates the entire development environment:

*   Starts all infrastructure services (PostgreSQL, Redis, NATS), sets up the database, runs pending migrations, and then runs the API service with hot-reloading (if Air is installed) and debugging capabilities (Delve on port `2345`).

```bash
make dev
```

Once running:
*   **API Endpoint**: `http://localhost:8080`
*   **Swagger UI**: `http://localhost:8080/docs` (for API documentation)
*   **Delve Debugger**: Connect your debugger to `localhost:2349` (if `make install-dev-tools` was run).

To stop all services started by `make dev`:

```bash
make dev-down
```

### Manually Managing Services

For more granular control, you can manage services step-by-step:

*   **Start All Docker Services (Infrastructure + Application)**:
    ```bash
    make docker-up
    ```
    If this is a clean start or you've modified database schemas, you will need to run migrations *after* the services are up:
    ```bash
    make migrate-up
    ```
*   **Start Infrastructure Services Only**: 
    ```bash
    make dev-infra
    ```
*   **Create Database**: 
    ```bash
    make dev-db-setup
    ```
*   **Apply Database Migrations**: 
    ```bash
    make migrate-up
    ```
*   **Build and Run API Manually (No Hot-Reload/Debug)**: 
    ```bash
    make build && ./bin/api
    ```
*   **Stop All Docker Services**: 
    ```bash
    make docker-down
    ```

## Building Services

To build all Go services (api, notifier, rating-worker) into the `bin/` directory:

```bash
make build
```

## Testing

*   **Run All Unit Tests** (with race detector and coverage):
    ```bash
    make test
    ```
*   **Run Integration Tests** (requires Docker services to be running):
    ```bash
    make test-integration
    ```
    Note: The current integration test setup is primarily designed for local developer convenience. Running these tests in a CI/CD pipeline would require a more thoughtful and potentially optimized setup to manage the lifecycle of dependent Docker services.

## API Documentation

Generate and view the Swagger/OpenAPI documentation:

1.  **Generate Docs**:
    ```bash
    make swagger
    ```
2.  **Access UI**: Start the API (`make dev`) and navigate to `http://localhost:8080/docs` in your browser.

## Debugging

If you ran `make install-dev-tools` and then `make dev`, you can attach any Delve-compatible debugger to `localhost:2349` to set breakpoints and step through your Go code.

## Viewing Logs

To view logs for individual services when running via `docker-compose` (e.g., `make dev` or `make docker-up`):

*   **API Logs**: `docker-compose logs -f api`
*   **Notifier Logs**: `docker-compose logs -f notifier`
*   **Rating Worker Logs**: `docker-compose logs -f rating-worker`
