# eCommerce Backend – Production‑Grade Go Utils & Sample Microservice

This repository provides **reusable, production‑ready Go packages** for common e‑commerce domain concerns, plus a complete **sample order service** that demonstrates their usage. The service uses **Gin** for HTTP, **PostgreSQL** for persistence, and includes full unit & integration tests.

---

## 📦 Utils Packages (Domain Concerns)

### 1. `money` – Monetary Calculations with Currency Configuration

**What it does:**  
- Represents monetary values as a **value object** (`Money`) with amount (`decimal.Decimal`) and currency (`string`).  
- Prevents floating‑point errors by using `github.com/shopspring/decimal`.  
- Enforces currency compatibility during arithmetic operations (`Add`, `Sub`, `Mul`).  
- Automatically **rounds** according to each currency’s precision (e.g., USD → 2 decimals, JPY → 0 decimals).  
- Allows registration of custom currencies with configurable precision.  

**Key functions:**  
- `NewMoney(amount decimal.Decimal, currency string) (Money, error)` – constructor with validation.  
- `(m Money) Add(other Money) (Money, error)` – returns a new Money, errors on currency mismatch.  
- `(m Money) Round() Money` – rounds amount to currency’s required decimal places.  
- `IsValidCurrency(code string) bool` – checks if currency is supported.  
- `RegisterCurrency(cfg CurrencyConfig)` – adds or overrides a currency (e.g., for crypto).  

**Used in:** Order total calculation, line‑item pricing.

---

### 2. `timeutil` – Time Handling (UTC Storage, TZ Conversion, Monotonic Clocks)

**What it does:**  
- Enforces **UTC storage** for all timestamps (database, APIs).  
- Provides **safe timezone conversion** from UTC to a user’s local time at the presentation layer.  
- Uses the **monotonic clock** for measuring elapsed durations (e.g., request latency).  
- Includes a `Clock` interface for **testable time** (mock `Now()`).  

**Key functions:**  
- `NowUTC() time.Time` – current UTC time.  
- `ConvertToTimeZone(t time.Time, loc *time.Location) time.Time` – converts UTC to a given location.  
- `MonotonicElapsed(start time.Time) time.Duration` – uses `time.Since` (monotonic).  
- `Clock` interface + `FixedClock(t time.Time)` – for deterministic testing.  

**Used in:** Order creation timestamps (`CreatedAt`, `UpdatedAt`), graceful shutdown timeouts.

---

### 3. `id` – ULID Generation with Ordering Semantics

**What it does:**  
- Generates **time‑ordered, lexicographically sortable identifiers** (ULID by default).  
- Embeds a **48‑bit timestamp (milliseconds)** as the prefix → IDs are naturally ordered by creation time.  
- Supports **monotonic entropy** → guarantees total order even for IDs created in the same millisecond.  
- Provides timestamp extraction from an ID without an extra database column.  

**Key functions:**  
- `NewID() string` – generates a new ULID (default generator).  
- `ParseTime(idStr string) (time.Time, error)` – extracts the creation time from any ULID.  

**Why not UUIDv4 or auto‑increment?**  
- UUIDv4 is random → destroys database index locality.  
- Auto‑increment requires a centralised sequence → scaling bottleneck in distributed systems.  
- ULID is **distributed, sortable, URL‑safe, and dependency‑free** (pure Go).  

**Used in:** Order ID generation.

---

### 4. `validation` – Domain Invariants & Constructor Helpers

**What it does:**  
- Provides simple, reusable validation functions for **domain invariants** (e.g., non‑empty strings, positive numbers).  
- Collects multiple errors into an `ErrorList` for batch validation.  
- Works seamlessly with constructor functions (e.g., `NewOrder`) to prevent creation of invalid aggregates.  

**Key functions:**  
- `ValidatePositive(val int64, fieldName string) error` – ensures >0.  
- `ValidateNonZeroString(s, fieldName string) error` – rejects empty/whitespace strings.  
- `ValidateMinLength(s, fieldName string, min int) error` – enforces minimum length.  
- `ValidateCurrencyCode(code string, validator CurrencyValidator, fieldName string) error` – pluggable currency check.  
- `ErrorList` – aggregates multiple validation errors into one.  

**Used in:** Order constructor validation (customer ID, items quantity, product IDs).

---

## 🛠️ Sample Microservice – Order Service

The **order‑service** is a production‑grade HTTP API that uses all the utils packages.

### Architecture Overview




5. Run the service
bash
cd sampleApp/cmd
go run main.go
Or using the built binary:

bash
go build -o order-service ./sampleApp/cmd
./order-service
The server will start on http://localhost:8080.

🧪 Running Tests
Unit Tests (mock repository, no DB)
bash
go test -v -cover ./...
Integration / Functional Tests (with Docker)
The repository includes a script run_test.sh that:

Starts a PostgreSQL container

Runs migrations

Builds the service

Starts the service

Executes API end‑to‑end tests (create, get, confirm, cancel)

Cleans up

bash
chmod +x run_test.sh
./run_test.sh
📡 API Endpoints & How They Use Utils
All endpoints use query parameters for id (except POST /orders which uses JSON body).

Method	Endpoint	Description
POST	/orders	Create a new order
GET	/orders?id={id}	Retrieve an order by ID
POST	/orders/confirm?id={id}	Confirm an order (PENDING → CONFIRMED)
POST	/orders/cancel?id={id}	Cancel an order (PENDING → CANCELLED)
Detailed Walkthrough & Utils Integration
POST /orders – Create Order
What it does:

Validates input (customer ID, items non‑empty, positive quantities, valid prices).

Ensures all items share the same currency.

Calculates the total amount using money.Add and money.Mul.

Rounds the total according to currency precision.

Generates a ULID for the order ID.

Stores UTC timestamps for created_at and updated_at.

Persists the order and its items in PostgreSQL.

How utils are used:

validation.ValidateNonZeroString, ValidatePositive – enforce domain invariants.

money.NewMoney, Add, Mul, Round – safe arithmetic & rounding.

id.NewID() – generate time‑ordered, sortable ID.

timeutil.NowUTC() – store timestamps in UTC.

Example request:

bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customer_id": "cust_123",
    "items": [
      {"product_id": "p1", "quantity": 2, "price": "19.99", "currency": "USD"},
      {"product_id": "p2", "quantity": 1, "price": "5.50", "currency": "USD"}
    ]
  }'
Example response (201 Created):

json
{
  "id": "01J3M6QZ7X2A4B5C6D7E8F9G0H",
  "customer_id": "cust_123",
  "total": "USD 45.48",
  "status": "PENDING",
  "created_at": "2025-03-15T10:30:00Z",
  "updated_at": "2025-03-15T10:30:00Z"
}
GET /orders?id={id} – Retrieve Order
What it does:

Fetches the order and its items from PostgreSQL.

Reconstructs money.Money objects from stored amount/currency.

Returns the order as JSON.

How utils are used:

id.ParseTime(id) – can be used to extract creation time from the ID (optional).

money.NewMoney – reconstructs monetary values from database columns.

Example request:

bash
curl "http://localhost:8080/orders?id=01J3M6QZ7X2A4B5C6D7E8F9G0H"
Example response (200 OK):
(same as above)

POST /orders/confirm?id={id} – Confirm Order
What it does:

Finds the order by ID.

Checks that current status is PENDING.

Changes status to CONFIRMED.

Updates updated_at to current UTC time.

Persists the change.

How utils are used:

timeutil.NowUTC() – updates the updated_at timestamp.

(indirectly) id.NewID() and validation are used during order creation.

Example request:

bash
curl -X POST "http://localhost:8080/orders/confirm?id=01J3M6QZ7X2A4B5C6D7E8F9G0H"
Response: 204 No Content (or 400 Bad Request if already confirmed/cancelled)

POST /orders/cancel?id={id} – Cancel Order
What it does:

Finds the order by ID.

Checks that current status is PENDING.

Changes status to CANCELLED.

Updates updated_at to current UTC time.

Persists the change.

How utils are used:

Same as confirm – timeutil.NowUTC() for timestamp.

Example request:

bash
curl -X POST "http://localhost:8080/orders/cancel?id=01J3M6QZ7X2A4B5C6D7E8F9G0H"
Response: 204 No Content (or 400 Bad Request if not pending)
