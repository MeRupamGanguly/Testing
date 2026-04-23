Below is the complete, updated `README.md` with a new **Error Handling** section that explains how Go manages exceptions without Java references.

---

# Crosscutting API

A production-ready Go web API built with **Gin** that demonstrates essential cross-cutting concerns—structured logging, JWT authentication, rate limiting, input validation, panic recovery, and consistent error responses. The project is self-contained, containerised, and comes with an automated integration test suite.

## Table of Contents

- [Overview](#overview)
- [Project Structure](#project-structure)
- [Module & Dependencies](#module--dependencies)
- [File-by-File Breakdown](#file-by-file-breakdown)
  - [context_keys.go](#context_keysgo)
  - [response.go](#responsego)
  - [validator.go](#validatorgo)
  - [auth.go (middleware)](#authgo-middleware)
  - [logging.go (middleware)](#logginggo-middleware)
  - [ratelimit.go (middleware)](#ratelimitgo-middleware)
  - [recovery.go (middleware)](#recoverygo-middleware)
  - [main.go](#maingo)
  - [run_test.sh](#run_testsh)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Running Locally](#running-locally)
  - [Running Tests](#running-tests)
- [API Endpoints](#api-endpoints)
- [Configuration](#configuration)
- [Error Handling](#error-handling)
- [Design Decisions](#design-decisions)

## Overview

**Crosscutting API** is a reference implementation that bundles reusable middleware and utility packages into a single Go module. It shows how to:

- Enforce **JWT-based authentication** and role-based access control.
- Apply **rate limiting** (in-memory token bucket or Redis-backed leaky bucket).
- Log structured, request-scoped information using `log/slog`.
- Recover from panics gracefully.
- Normalise all API responses in a consistent JSON envelope.
- Validate request payloads with custom business rules.
- Containerise and test the service end-to-end with a shell script.

The code is intended for learning, bootstrapping new services, or dropping into existing Gin applications that need these capabilities.

## Project Structure

```
.
├── go.mod                     # Module definition and dependencies
├── context_keys.go            # Typed context keys (utils package)
├── response.go                # Standard API response helpers (utils package)
├── validator.go               # Custom Gin validations (utils package)
├── auth.go                    # JWT authentication & role middleware
├── logging.go                 # Structured request logging middleware
├── ratelimit.go               # Rate limiting middleware (token/leaky bucket)
├── recovery.go                # Panic recovery middleware
├── main.go                    # Application entry point, routes, and wiring
└── run_test.sh                # End-to-end integration test script
```

All files reside in the same Go module (`crosscutting`). The middleware packages are located under `middleware/` in the original layout, but the provided file contents assume they are directly at the root or in `middleware/` subdirectories. The file paths in the code are illustrative (`pkg/middleware/auth.go` etc.). In practice, you can reorganise into traditional `cmd/` and `internal/` directories if desired.

## Module & Dependencies

**Module name:** `crosscutting`  
**Go version:** 1.26.1

Key direct dependencies:

| Package | Purpose |
|--------|---------|
| `github.com/gin-gonic/gin` | HTTP framework |
| `github.com/go-playground/validator/v10` | Struct validation |
| `github.com/golang-jwt/jwt/v5` | JWT signing & verification |
| `github.com/redis/go-redis/v9` | Redis client (optional, for distributed rate limiting) |
| `golang.org/x/time/rate` | In-memory token bucket rate limiter |

All indirect dependencies are listed in `go.mod`; they are pulled in automatically by the build process.

## File-by-File Breakdown

### `context_keys.go`

**Package:** `utils`  
**Purpose:** Defines typed context keys to avoid collisions when storing data in Gin’s `c.Set()/c.Get()` or Go contexts.

- Declares a custom type `contextKey` based on `string`.
- Four keys are defined: `RequestIDKey`, `UserIDKey`, `UserRoleKey`, `SessionIDKey`.
- Each key has a `String()` method, allowing it to be used as a map key or printed.

**Logic:** Middleware and handlers store values (like user ID, role, request ID) using these keys. The strong typing prevents accidental overwrites from different packages using the same string literal.

---

### `response.go`

**Package:** `utils`  
**Purpose:** Provides a consistent JSON envelope for all API responses and sends appropriate security headers.

- **`Envelope`** struct wraps success/data/error/meta fields.
- **`ErrorDetail`** carries a machine-readable code, human-readable message, and optional field name (for validation errors).
- **`HeaderPolicy`** sets `Content-Type`, `X-Content-Type-Options`, `X-Frame-Options`, `X-XSS-Protection`, and echoes the request ID.
- Helper functions (`OK`, `Created`, `NoContent`, `Error`, `ValidationError`, `Unauthorized`, `Forbidden`, `NotFound`, `InternalServerError`) encapsulate common status codes and error messages.

**Logic:** Every handler calls one of these helpers. They always call `HeaderPolicy` and then send the appropriate HTTP status with the standard envelope. This guarantees uniform responses and reduces boilerplate.

---

### `validator.go`

**Package:** `utils`  
**Purpose:** Registers custom validation tags for Gin’s built‑in validator and provides a standalone validation function.

- **`RegisterCustomValidations()`** – called once at startup. It:
  - Configures the validator to use JSON tag names in error messages.
  - Registers three custom rules: `sku` (alphanumeric, 6–20 chars), `price` (positive float), `phone` (starts with `+`, length 10–15).
- **`Validate(data interface{})`** – standalone helper that creates a new validator, registers the same rules, and returns a map of field‑error messages.
- **`formatError`** – converts validation tags into user-friendly messages.

**Logic:** The custom tags are used in struct definitions like `binding:"required,sku"`. When binding fails, `gin` returns validation errors; the error formatter produces clear messages.

---

### `auth.go` (middleware)

**Package:** `auth`  
**Purpose:** JWT authentication and optional role-based access control middleware for Gin.

- **`AuthConfig`** struct holds configuration: secret, token lookup method (`header`, `query`, `cookie`), auth scheme, excluded paths, and role requirements.
- **`Auth(config)`** returns a Gin middleware that:
  1. Skips excluded paths.
  2. Extracts the token according to `TokenLookup`.
  3. Parses and validates the JWT (HMAC-SHA256).
  4. Extracts `sub` (user ID) and `role` claims.
  5. Stores them in the Gin context using the typed keys from `utils`.
  6. If `RoleRequired` is true, checks that the role matches `AdminRoleValue` and aborts with 403 if not.
- **`RequireRole(requiredRole)`** is a separate middleware that checks the stored role and aborts with 403 if mismatched. Useful for protecting routes after authentication.

**Logic:** Allows fine-grained access control. A typical flow: authenticate with `Auth`, then protect admin routes with `RequireRole("admin")`.

---

### `logging.go` (middleware)

**Package:** `logging`  
**Purpose:** Structured request/response logging using `log/slog`.

- **`Logging(logger)`** returns a middleware that:
  - Reads or generates a request ID (`X-Request-ID` header) and stores it in context.
  - Records the start time, method, path, query, status code, client IP, latency, user agent, response size, and optionally user ID and errors.
  - Logs at `INFO` level normally, `WARN` for 4xx, `ERROR` for 5xx.
- **`generateRequestID`** – simple demo implementation using timestamp + random string (in production, prefer a UUID library).

**Logic:** Provides a complete, structured audit trail for every request. The log line is emitted after the request finishes, so it includes the final status code and latency.

---

### `ratelimit.go` (middleware)

**Package:** `ratelimit`  
**Purpose:** Rate limiting with two strategies: in‑memory token bucket and Redis-backed leaky bucket.

- **`RateLimitConfig`** aggregates configuration for both approaches.
- **`TokenBucketRateLimit(rate, burst)`** – per‑IP in‑memory token bucket.
  - Uses a `sync.RWMutex`‑protected map of `rate.Limiter` instances.
  - On denial, sets rate limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`) and returns HTTP 429.
- **`LeakyBucketRateLimit(config)`** – distributed leaky bucket using Redis.
  - Uses a Lua script to atomically update a hash storing `tokens` and `last_update`.
  - Refills tokens based on elapsed time and configured rate.
  - On denial, returns 429 with headers and consumes no token.
  - If Redis is unavailable, the middleware fails open (logs and continues) to avoid blocking all traffic.

**Logic:** The main application tries Redis first; if Redis is unreachable, it falls back to the in‑memory token bucket. This ensures a smooth degradation path from distributed to single‑instance rate limiting.

---

### `recovery.go` (middleware)

**Package:** `recovery`  
**Purpose:** Catches panics in handlers, logs the error and stack trace, and sends a 500 response.

- **`Recovery(logger)`** wraps the request with a deferred function.
- On panic, it logs the error and stack using `slog`, then calls `utils.InternalServerError` to send a safe 500 JSON response.
- Checks `c.Writer.Written()` to avoid duplicate headers if a partial response was already sent.

**Logic:** Prevents the server from crashing on unexpected panics and ensures a graceful response to the client.

---

### `main.go`

**Package:** `main`  
**Purpose:** Application entry point – configures, wires, and starts the HTTP server.

- Creates a structured JSON logger.
- Calls `utils.RegisterCustomValidations()` to set up custom validation rules.
- Initialises an optional Redis client from the `REDIS_ADDR` environment variable.
- Creates a Gin engine with `gin.New()` (no default middleware).
- Attaches middleware in order:
  1. `recovery.Recovery` – must be first to catch panics everywhere.
  2. `logging.Logging` – logs every request.
  3. Rate limiter – Redis leaky bucket if Redis is available, else in‑memory token bucket.
- Configures JWT auth using `JWT_SECRET` env var (default insecure fallback).
- Defines routes:
  - `/health` (public)
  - `/api/v1/profile` (authenticated)
  - `/api/v1/orders` (authenticated, validated payload)
  - `/api/v1/admin/users` (admin only)
- Starts the server on the port defined by `PORT` (default `8080`).

**Logic:** Demonstrates the complete integration of all middleware. The order of middleware is important: recovery → logging → rate limit → auth → handler.

---

### `run_test.sh`

**Purpose:** Automated end‑to‑end integration test script using **Podman** (Docker-compatible).

The script:

1. Cleans up previous containers and networks.
2. Generates JWT tokens for a customer and an admin using a hard‑coded secret.
3. Creates a custom Podman network and starts a Redis container.
4. Builds the API container from the current directory using a multi‑stage Dockerfile embedded in the script.
5. Starts the API container, connecting it to Redis, and waits for readiness.
6. Runs a series of `curl` tests covering:
   - Health check (200)
   - Unauthenticated request (401)
   - Authenticated request (200)
   - Invalid payload (400)
   - Valid order creation (201)
   - Customer accessing admin route (403)
   - Admin accessing admin route (200)
   - Rate limit enforcement (429 after >100 requests)
7. Reports pass/fail for each test and cleans up on exit.

**Logic:** Provides a single command to verify that all cross‑cutting concerns work together correctly. It requires Podman and `openssl` for JWT generation.

## Getting Started

### Prerequisites

- **Go 1.26.1** (or later) if you want to run natively.
- **Podman** (or Docker) with `docker.io` compatibility for the containerised test.
- `openssl` (usually pre‑installed) for JWT generation in tests.
- A Redis instance is optional; the service starts without it (falls back to in‑memory rate limiting).

### Running Locally

```bash
# Clone the repository
git clone <repo-url>
cd crosscutting

# Install dependencies
go mod download

# Set environment variables (optional)
export REDIS_ADDR=localhost:6379
export JWT_SECRET=super-secret
export PORT=8080

# Run the server
go run main.go
```

The API will be available at `http://localhost:8080`.

### Running Tests

Execute the integration test script:

```bash
chmod +x run_test.sh
./run_test.sh
```

The script builds a container image, starts Redis and the API, runs all checks, and prints a summary. It requires Podman to be installed and running.

## API Endpoints

| Method | Path                  | Auth Required | Description                         |
|--------|-----------------------|---------------|-------------------------------------|
| GET    | `/health`             | No            | Health check                        |
| GET    | `/api/v1/profile`     | Yes (any)     | Returns current user's profile      |
| POST   | `/api/v1/orders`      | Yes (any)     | Creates an order (validated body)   |
| GET    | `/api/v1/admin/users` | Yes (admin)   | Lists users (admin‑only)            |

**Request body for `POST /api/v1/orders`:**

```json
{
  "product_sku": "SKU12345",
  "quantity": 2,
  "price": 29.99
}
```

Validation rules: `product_sku` must be 6–20 alphanumeric characters; `quantity` >= 1; `price` > 0.

**Response envelope (success):**

```json
{
  "success": true,
  "data": { ... },
  "meta": { ... }
}
```

**Response envelope (error):**

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

## Configuration

The application uses environment variables:

| Variable      | Default               | Description                                         |
|---------------|-----------------------|-----------------------------------------------------|
| `JWT_SECRET`  | `change-me-in-production` | Secret key for signing/verifying JWT tokens.   |
| `REDIS_ADDR`  | `localhost:6379`      | Redis address. Leave empty to disable Redis.        |
| `PORT`        | `8080`                | HTTP server port.                                   |

For production, **always set a strong `JWT_SECRET`** and use Redis for distributed rate limiting.

## Error Handling

Go does not use exceptions; instead, errors are returned explicitly and handled through middleware. This application covers all error scenarios using a combination of custom response helpers, validation, and recovery middleware.

### Request Binding and Validation

Input validation occurs when handlers call `c.ShouldBindJSON`. If binding fails—whether due to type mismatch, missing required fields, or custom rules like `sku`—the error is captured and returned as a `400 Bad Request`:

```go
if err := c.ShouldBindJSON(&req); err != nil {
    utils.Error(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
    return
}
```

The custom validator (`validator.go`) translates tag-based rules into human-readable messages. For a more detailed response, you can extend this to return a list of field-specific errors (the standalone `utils.Validate` function already returns a map of field-to-message).

### Authentication and Authorization

The auth middleware (`auth.go`) handles missing or invalid tokens by calling `utils.Unauthorized` or `utils.Forbidden`. This mimics exception-based HTTP error responses but is controlled entirely through middleware logic:

- Missing token → `401 Unauthorized` with message "missing authentication token"
- Invalid/expired token → `401 Unauthorized`
- Insufficient role → `403 Forbidden`

All are returned via the standard `Envelope` structure.

### Global Panic Recovery

Unhandled panics (which would otherwise crash the server) are caught by the recovery middleware (`recovery.go`). It logs the error and stack trace, then responds with a `500 Internal Server Error` using `utils.InternalServerError`. This provides a safety net equivalent to a global error handler but without hidden control flow.

### Structured Error Responses

All error responses follow the same JSON envelope defined in `response.go`. Each response includes `success: false`, an `error` object with `code`, `message`, and optional `field`, and automatically receives security headers. This consistent format simplifies error handling on the client side.

### Further Enhancements

While the current implementation covers all critical error paths, you could enrich error responses by:

- Extracting multiple validation errors into a `details` array.
- Including a timestamp and request path in the error envelope.
- Mapping specific binding error types to custom error codes.

The explicit approach in Go makes error handling transparent and easy to reason about without hidden control structures.

## Design Decisions

- **Typed context keys** prevent collisions between packages.
- **Standardised response envelope** simplifies client-side error handling.
- **Custom validators** are registered once at startup and used declaratively in struct tags.
- **Middleware layering** follows best practices: recovery first, then logging, then rate limiting, then authentication.
- **Graceful degradation** of rate limiting: if Redis is unavailable, the service continues with an in‑memory limiter (suitable for single‑replica deployments).
- **All tests are automated** via a shell script, making CI integration straightforward.
- **Slog** is used for structured logging, emitting JSON to stdout by default—easy to ingest into log aggregators.

---

Feel free to extend or reorganise the code; the module structure is intentionally flat to keep the example concise. For larger projects, consider moving middleware into `internal/middleware/` and utilities into `internal/utils/`.
