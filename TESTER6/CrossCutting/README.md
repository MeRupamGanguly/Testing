# Crosscutting API

A production‑ready e‑commerce API built with Go and Gin, showcasing industry‑standard cross‑cutting concerns including structured logging, distributed tracing, Prometheus metrics, JWT authentication, rate limiting, panic recovery, and standardised API responses.

## Features

- **Structured Logging** – JSON logs with request ID, latency, status, and user context using `log/slog`.
- **Distributed Tracing** – OpenTelemetry integration with OTLP exporter (Jaeger compatible).
- **Prometheus Metrics** – Request counts, durations, and in‑flight requests.
- **JWT Authentication** – HMAC‑signed tokens with role‑based access control (RBAC).
- **Rate Limiting** – Distributed leaky bucket (Redis) with fallback to in‑memory token bucket.
- **Panic Recovery** – Graceful handling with stack traces logged.
- **Standardised Responses** – Consistent JSON envelope with security headers.
- **Custom Validation** – SKU, price, and phone number validators registered with Gin.



## Core Components



### `utils/context_keys.go`
Declares typed context keys to avoid collisions when storing values in `gin.Context`. Keys include `RequestIDKey`, `UserIDKey`, `UserRoleKey`, `SessionIDKey`, and `TraceIDKey`.

### `utils/response.go`
Provides a standard JSON response envelope (`Envelope`) with `success`, `data`, `error`, and `meta` fields. Includes helper functions for common HTTP statuses (`OK`, `Created`, `NoContent`, `Error`, `ValidationError`, `Unauthorized`, etc.) and a `HeaderPolicy` that sets security headers (`X-Content-Type-Options`, `X-Frame-Options`, etc.) and the request ID.

### `utils/validator.go`
Registers custom validation tags (`sku`, `price`, `phone`) with Gin’s validator. Maps struct tags to JSON field names for user‑friendly error messages. Also exports a `Validate` function for programmatic validation outside Gin binding.

### `middleware/auth.go`
JWT authentication middleware that extracts tokens from headers, query parameters, or cookies. Validates the token using a secret key and stores `user_id` and `role` in the context. Supports path exclusion and optional role‑based access control via the `RequireRole` helper.

### `middleware/logging.go`
Structured logging middleware using `log/slog`. Generates a request ID if not provided, logs request/response metadata after processing, and uses appropriate log levels based on HTTP status (≥500 = error, ≥400 = warn).

### `middleware/metrics.go`
Exposes Prometheus metrics for HTTP requests:
- `http_requests_total` – counter partitioned by method, path, and status.
- `http_request_duration_seconds` – histogram of request latencies.
- `http_requests_in_flight` – gauge of concurrent requests.

### `middleware/ratelimit.go`
Implements two rate limiting strategies:
- `TokenBucketRateLimit` – in‑memory token bucket per client IP (suitable for single‑instance deployments).
- `LeakyBucketRateLimit` – distributed leaky bucket using Redis and a Lua script (recommended for multi‑instance setups).

### `middleware/recovery.go`
Recovers from panics, logs the error and stack trace using `slog`, and returns a 500 Internal Server Error if headers haven’t been sent yet.

### `middleware/tracing.go`
Wraps the `otelgin` middleware to start a trace span for each request. Adds custom attributes such as `request.id` and `user.id` to the span. Stores the trace ID in the context for log correlation.

### `main.go`
Assembles the application:
- Initialises the logger, tracer provider, Redis client, and Gin router.
- Registers custom validations.
- Applies middleware in order: Recovery → Logging → Tracing → Metrics → Rate Limiting.
- Sets up public endpoints (`/health`, `/metrics`) and protected API routes under `/api/v1`.
- Demonstrates example handlers (`getProfile`, `createOrder`, `listUsers`).

### `run_test.sh`
A comprehensive integration test script using **Podman** (Docker‑compatible). It:
1. Spins up Redis and Jaeger containers.
2. Builds the API container using an inline `Dockerfile`.
3. Generates JWT tokens for a customer and an admin using `openssl`.
4. Runs `curl` tests against the live API to verify:
   - Health and metrics endpoints.
   - Authentication (401 without token, 200 with token).
   - Input validation (400 for invalid payload).
   - Role‑based access (403 for customer accessing admin routes).
   - Rate limiting (expects 429 after 101 rapid requests).
   - Tracing (checks Jaeger for recorded traces).
5. Cleans up all containers and networks after execution.

## Prerequisites

- **Go 1.26+** (for local development)
- **Podman** or **Docker** (for running the test script and optional dependencies)
- **Redis** (optional; used for distributed rate limiting)
- **Jaeger** (optional; for trace visualisation)

## Configuration

The application is configured via environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | `8080` |
| `REDIS_ADDR` | Redis server address (e.g., `localhost:6379`) | `localhost:6379` |
| `JWT_SECRET` | Secret key for JWT signing | `change-me-in-production` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OTLP endpoint for traces (e.g., `http://jaeger:4317`) | none |

## Running Locally

```bash
# Clone the repository
git clone <repository-url>
cd crosscutting

# Install dependencies
go mod download

# (Optional) Start Redis and Jaeger locally
podman run -d --name redis -p 6379:6379 redis:7-alpine
podman run -d --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:latest

# Set required environment variables
export JWT_SECRET="your-secret-key"
export REDIS_ADDR="localhost:6379"
export OTEL_EXPORTER_OTLP_ENDPOINT="http://localhost:4317"

# Run the application
go run main.go
```

## Testing with the Script

The `run_test.sh` script performs a complete end‑to‑end test using containers. It requires **Podman** and **jq** (optional, for tracing verification).

```bash
# Make the script executable
chmod +x run_test.sh

# Run the tests (ensure no conflicting services run on ports 8080/16686)
./run_test.sh
```

**What the script tests:**

- **Health endpoint** (`GET /health`) returns 200.
- **Metrics endpoint** (`GET /metrics`) is exposed and returns Prometheus metrics.
- **Authentication** – protected routes return 401 without a token.
- **Valid JWT** – customer token grants access to profile and order creation.
- **Input validation** – malformed order payload triggers a 400 with validation errors.
- **Admin RBAC** – customer cannot access `/admin/users` (403); admin can (200).
- **Rate limiting** – after 101 requests to `/api/v1/profile`, the API returns 429.
- **Tracing** – checks Jaeger UI API for recorded traces from the `ecommerce-api` service.

All containers and networks are automatically cleaned up on script exit.

## API Endpoints

| Method | Path | Description | Auth Required | Role |
|--------|------|-------------|---------------|------|
| GET | `/health` | Liveness probe | No | - |
| GET | `/metrics` | Prometheus metrics endpoint | No | - |
| GET | `/api/v1/profile` | Get current user profile | Yes | Any |
| POST | `/api/v1/orders` | Create a new order | Yes | Any |
| GET | `/api/v1/admin/users` | List all users (admin only) | Yes | `admin` |

**Example Request: Create Order**

```http
POST /api/v1/orders HTTP/1.1
Authorization: Bearer <JWT>
Content-Type: application/json

{
  "product_sku": "SKU12345",
  "quantity": 2,
  "price": 29.99
}
```

**Success Response (201 Created):**
```json
{
  "success": true,
  "data": {
    "order_id": "ord_123",
    "user_id": "customer-123"
  }
}
```

**Error Response (400 Bad Request):**
```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "must be a valid SKU (6-20 alphanumeric)",
    "field": "product_sku"
  }
}
```

## Middleware Pipeline

The Gin engine executes middleware in the following order:

1. **Recovery** – catches panics and prevents server crashes.
2. **Logging** – generates request ID and logs request/response details.
3. **Tracing** – starts OpenTelemetry span and propagates trace context.
4. **Metrics** – records Prometheus metrics for the request.
5. **Rate Limiting** – enforces request limits (Redis leaky bucket or in‑memory).
6. **Authentication** (route group) – validates JWT and extracts user info.
7. **Role‑Based Access Control** (route group) – enforces `admin` role where required.

## Dependencies

Key libraries used:

- **[gin-gonic/gin](https://github.com/gin-gonic/gin)** – HTTP web framework.
- **[golang-jwt/jwt](https://github.com/golang-jwt/jwt)** – JWT parsing and validation.
- **[go-playground/validator](https://github.com/go-playground/validator)** – Struct validation.
- **[redis/go-redis](https://github.com/redis/go-redis)** – Redis client for rate limiting.
- **[prometheus/client_golang](https://github.com/prometheus/client_golang)** – Prometheus metrics instrumentation.
- **[go.opentelemetry.io/otel](https://github.com/open-telemetry/opentelemetry-go)** – Distributed tracing.
- **[golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)** – Token bucket rate limiter.

See `go.mod` for the complete list.

## License

This project is provided for educational purposes. Feel free to use and adapt it as needed.
