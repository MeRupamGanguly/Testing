## Overview of the Two Modules

The codebase contains **two distinct but cooperating modules**:

1. **`LDKillSwitch`** – a reusable Go library that wraps the LaunchDarkly (LD) server SDK. It adds:
   - In‑memory caching with TTL (using `go-cache`)
   - Optional Redis persistent data store
   - Offline mode (local JSON file as source of truth)
   - Dynamic log‑level management (polls a string flag and adjusts `slog` level)

2. **`SampleApp`** – a demonstration application (simulated e‑commerce location service) that uses the `LDKillSwitch` library. It shows:
   - How to inject the feature‑flag service into business logic
   - A resilient HTTP client with **retries** and **circuit breaker** (via `gobreaker`)
   - A controller that checks a kill‑switch before calling a downstream API
   - Integration with the dynamic log‑level manager

---

## Why Do We Need These Modules?

### For `LDKillSwitch` (the library)

LaunchDarkly’s Go SDK is already powerful, but in high‑throughput microservices, every call to `BoolVariation` or `StringVariation` adds **network latency** (even though the SDK uses streaming updates). The library solves three pain points:

1. **Performance** – A local cache returns a flag value in **microseconds** instead of milliseconds.  
   *Why?* Many services evaluate the same flag for the same user context repeatedly (e.g., on every HTTP request). Caching avoids redundant SDK evaluations.

2. **Resilience** – If LaunchDarkly is temporarily unreachable, the cache still holds the last known value.  
   *Why?* Your service does not fail open or fall back to default on every request during a network blip.

3. **Offline Development / Testing** – By reading a `flags.json` file, developers can work without an internet connection or an LD account.  
   *Why?* Integration tests become deterministic and can run in CI without external dependencies.

4. **Dynamic Log Level** – A string flag (e.g., `"log-level"`) can change the application’s logging verbosity at runtime without redeploying.  
   *Why?* Debugging production issues often requires temporarily enabling `DEBUG` logs. A flag lets you do that instantly.

### For `SampleApp` (the demonstration)

This app shows a **real‑world pattern**: a location API that calls an internal “mock target” only if a LaunchDarkly kill‑switch allows it.  
It also demonstrates **resilience patterns** (retries + circuit breaker) that are often used alongside feature flags to build robust microservices.

> In essence: `SampleApp` is a teaching tool that uses `LDKillSwitch` correctly. You can copy its patterns into your own services.

---

## What Happens With vs Without an SDK Key?

The behaviour is controlled by the `offline` flag in `config.yaml` and the presence of an SDK key.

### 1. **With an SDK Key (Live Mode)**
- `ld_client.go` creates a **real LaunchDarkly client** using `ldclient.MakeCustomClient`.
- The client connects to LD’s streaming endpoint and receives flag updates in near real‑time.
- If `redis.enabled` is `true`, the SDK uses Redis as a **persistent data store** – all service instances share the same flag rules, reducing calls to LD.
- The local in‑memory cache (`go-cache`) is still used on top of the SDK to avoid calling `BoolVariation` too often.

**Flow (live mode):**  
`Application → Cache (go‑cache) → LaunchDarkly SDK → (if miss) LD servers or Redis`

### 2. **Without an SDK Key (Offline Mode)**
- The library checks `props.OfflineMode` or `props.SdkKey == ""`.
- It configures the LD client with `ldfiledata.DataSource().FilePaths("flags.json")`.
- The SDK never contacts LaunchDarkly – it reads and evaluates flag rules **locally** from the JSON file.
- The in‑memory cache still works (caching between evaluations of the same flag+context).

**Flow (offline mode):**  
`Application → Cache → LaunchDarkly SDK (reads flags.json) → no network calls`

> **Important:** Even in offline mode, the SDK **evaluates targeting rules** (e.g., `"targets": [{"values": ["LOC123"], "variation": 0}]`) correctly. The `flags.json` in the repository shows an example with a target for `LOC123`.

---

## How to Test Both Modules

### Testing the `LDKillSwitch` Library

The library comes with **unit tests** that use LaunchDarkly’s `testhelpers/ldtestdata` – a mock data source.

**Example – `cache_test.go`**:
- `TestGetBooleanFlagValue_CacheMiss` – verifies that a miss calls the mock client and stores the value.
- `TestGetBooleanFlagValue_CacheHit` – checks that a second call returns the cached value even if the underlying flag changes.
- `TestGetBooleanFlagValue_CacheDisabled` – ensures that when `CacheEnabled = false`, every call goes to the SDK.

**Run library tests** (from the module root):
```bash
cd LDKillSwitch
go test -v ./...
```

**What they mock**:  
`ldtestdata.DataSource()` simulates a LaunchDarkly server. You can programmatically set flag variations with `td.Flag(...).VariationForAll(...)`.

### Testing the `SampleApp`

The sample app has three layers of tests:

1. **Client tests** (`client_test.go`) – test the HTTP retry and circuit‑breaker logic using `httptest.NewServer`.
2. **Service tests** (`serviceapp_test.go`) – test the business logic that checks the kill‑switch before calling the downstream API. They also use `ldtestdata` to mock LaunchDarkly.
3. **Integration / manual testing** – run the app with `go run ./SampleApp/cmd/main.go` and send HTTP requests.

**Manual test steps** (offline mode, because `offline: true` in `config.yaml`):

```bash
# Start the app
go run ./SampleApp/cmd/main.go

# In another terminal:
# 1. Kill‑switch allows LOC123 (see flags.json – target for "LOC123" returns false)
curl "http://localhost:8080/locations/LOC123/booleantest"
# Expected: 400 Bad Request with "feature_disabled_by_killswitch"

# 2. Kill‑switch allows any other ID (fallthrough variation = true)
curl "http://localhost:8080/locations/OTHER/booleantest"
# Expected: 200 OK with the mock API response

# 3. Test dynamic log level
# Initially log level is ERROR (fallthrough variation 3 -> "ERROR").
# Change flags.json: set "log-level" fallthrough variation to 0 ("DEBUG").
# Wait up to 30 seconds (poll interval) – you will see DEBUG logs appear.
```

**Testing live mode** (with a real SDK key):
- Set `offline: false` in `config.yaml` and provide a valid `sdkKey`.
- Remove `flags.json` or keep it (offline mode overrides).
- Run the app – it will connect to LaunchDarkly. You can change flag values in the LD dashboard and see the kill‑switch take effect within a few seconds (plus cache TTL).

---

## Summary Table

| Scenario                  | SDK Key? | offline flag | Data Source         | Caching | Network calls to LD? |
|---------------------------|----------|--------------|---------------------|---------|----------------------|
| Live mode                 | Yes      | false        | LD servers (+Redis) | Yes     | Yes (streaming)      |
| Offline mode (dev/test)   | Any      | true         | flags.json          | Yes     | No                   |
| No SDK key (auto offline) | Empty    | false        | flags.json          | Yes     | No                   |

**Key takeaway:** The library gives you a **uniform API** (`IsFeatureEnabled`, `GetStringFlag`) regardless of whether you are online or offline. The cache and the dynamic log manager work in both modes, making your service both fast and testable.

---

## Additional Learning Points

- **Cache key design** – The library uses `"bool:" + flagKey + ":" + ctx.Key()` to differentiate flags and user contexts. That’s correct because the same flag may evaluate differently for different users (e.g., beta testers).
- **Thread safety** – `go-cache` is safe for concurrent use, and the atomic `level` in `LogLevelManager` ensures race‑free log level changes.
- **Graceful shutdown** – `main.go` closes the LD client and stops the log manager’s poll loop.
- **Resilience** – The sample’s HTTP client demonstrates **retry with exponential backoff** and a **circuit breaker** – a perfect complement to kill‑switches (fail fast when downstream is sick).

If you want to **reuse the library** in your own project, simply import `featuresgflags/LDKillSwitch/cache`, `service`, and `config`, then follow the pattern shown in `main.go`. The library is self‑contained and has no external dependencies beyond LaunchDarkly’s SDK and `go-cache`.



## 1. Overview of the Two Modules

The codebase is split into **two cooperating modules**:

- **`LDKillSwitch`** – a reusable Go library that wraps the LaunchDarkly (LD) server SDK. It adds in‑memory caching, Redis persistence, offline mode, and dynamic log‑level management.
- **`SampleApp`** – a demonstration application (simulated location service) that uses the `LDKillSwitch` library. It shows kill‑switch patterns, resilient HTTP clients (retries + circuit breaker), and integration with dynamic logging.

Together they form a **production‑ready template** for using LaunchDarkly in high‑throughput microservices.

---

## 2. What Each File Does (by module)

### `LDKillSwitch` Library

| File | Purpose |
|------|---------|
| `cache.go` | Implements `FeatureFlagCache` – an in‑memory TTL cache (using `patrickmn/go-cache`) that stores evaluated flag values. Methods: `GetBooleanFlagValue`, `GetStringFlagValue`, `GetJSONFlagValue`. Each cache key includes flag key + context key (e.g. `"bool:flagX:user123"`). |
| `cache_test.go` | Unit tests for the cache layer using LaunchDarkly’s `testhelpers/ldtestdata` (mock data source). Tests cache hit/miss, cache disabled, fallback to default. |
| `config.go` | Simple structs for LaunchDarkly client config (`SdkKey`, `OfflineMode`) and cache config (`CacheEnabled`, `ExpireAfterWriteMinute`). |
| `ld_client.go` | Factory function `NewLDClient`. Decides between **live mode** (real LD with optional Redis persistent store) and **offline mode** (local `flags.json` file using `ldfiledata`). Returns an `*ldclient.LDClient`. |
| `service.go` | `FeatureFlagService` – a thin wrapper over `FeatureFlagCache`. Provides clean API (`IsFeatureEnabled`, `GetStringFlag`, `GetJSONFlag`). This is what your application code should call. |
| `service_test.go` | Unit tests for `FeatureFlagService` using the mock data source. |
| `log_manager.go` | Implements `LogLevelManager` – polls a JSON flag (e.g. `"log-levels"`) that returns a list of allowed log levels (e.g. `["DEBUG","INFO"]`). Dynamically adjusts the global `slog` minimum level. Uses an atomic value and a polling goroutine. |
| `go.mod` | Module dependencies. Includes LaunchDarkly SDK v7, `go-cache`, Redis integration, etc. |
| `flags.json` | Example offline flag definitions. Contains `"location-feature-flag"` (boolean) with a target for `LOC123` returning `false`, and `"log-levels"` (JSON array) with four variations representing different sets of allowed levels. |
| `README.md` | High‑level documentation of the library’s features, architecture, and usage. |
| `Notes.md` | Detailed explanation of why the modules exist, how offline vs live mode works, and how to test both. (Written by the developer for internal understanding.) |

### `SampleApp` Demonstration

| File | Purpose |
|------|---------|
| `main.go` | Entry point. Loads `config.yaml`, initialises LD client, cache, feature flag service, and log manager. Starts dynamic log level polling. Sets up Gin router and graceful shutdown. |
| `configapp.go` | Defines `AppConfig` struct and loads `config.yaml`. Includes nested config for HTTP client (retry, circuit breaker), services, LD kill‑switch, and Redis. |
| `config.yaml` | Actual configuration file. Shows offline mode enabled, cache TTL 2 minutes, Redis optional, circuit breaker thresholds, etc. |
| `controller.go` | Gin controllers: `/locations/:id/booleantest` (simple kill‑switch test) and `/locations/:id/booleanattributestest` (with query params for context attributes). Calls `WebClientService.Get`. |
| `serviceapp.go` | `WebClientService` – business logic. **Before calling** the downstream API, it checks `featureFlagService.IsFeatureEnabled` with the given context. If the flag is `false`, it returns an error `feature_disabled_by_killswitch`. |
| `serviceapp_test.go` | Unit tests for `WebClientService`. Uses mock LD data source and `httptest` to verify kill‑switch blocking and success paths. |
| `client.go` | `WebClient` – an HTTP client wrapper with **retries** (exponential backoff) and a **circuit breaker** (using `sony/gobreaker`). `NewWebClientManager` builds a client per configured service. |
| `client_test.go` | Unit tests for retry logic and circuit breaker behaviour using `httptest` servers. |

---

## 3. What We Are Achieving Together

By combining the two modules, we achieve a **production‑grade feature flag pipeline**:

1. **Low‑latency flag evaluation** – The in‑memory cache (`go-cache`) serves repeated requests for the same flag+context in microseconds. Only cache misses call the LD SDK.

2. **Kill‑switch pattern** – The sample app demonstrates a **circuit breaker for business logic**: before calling an internal API (the “mock target”), it checks a LaunchDarkly boolean flag. If the flag is `false`, the call is blocked. This allows operations to instantly disable a feature without redeploying.

3. **Resilience** – The sample’s HTTP client includes retries (with backoff) and a circuit breaker. Combined with the kill‑switch, the service can gracefully handle downstream failures **and** external feature toggles.

4. **Dynamic log levels** – The `LogLevelManager` polls a JSON flag (`"log-levels"`) every 30 seconds. You can change the set of allowed log levels (e.g. `["DEBUG","INFO","WARN","ERROR"]` → `["ERROR"]`) at runtime, and the application will start logging more or less verbosely. This is invaluable for debugging production incidents.

5. **Offline development** – Developers can run the whole stack without an internet connection or a LaunchDarkly account. The library reads `flags.json` and the SDK evaluates targeting rules locally. All tests are deterministic and fast.

6. **Production readiness** – The library supports Redis as a **persistent data store** for the LD SDK. This reduces network calls to LaunchDarkly’s servers and keeps flag data synchronised across many service instances.

---

## 4. LaunchDarkly Flags – What to Set and How

The code uses two flags. Here’s how you would configure them in the LaunchDarkly dashboard (or in `flags.json` for offline mode).

### Flag 1: `location-feature-flag` (boolean)

**Purpose:** Kill‑switch for the location service API.  
**Type:** Boolean.  
**Variations:** `[false, true]` (standard order).  
**Targeting rules (example):**

| Rule | Variation |
|------|-----------|
| If `context.key` equals `"LOC123"` → return `false` (block) | Variation 0 |
| Otherwise → return `true` (allow) | Variation 1 (fallthrough) |

**In `flags.json` (offline):**
```json
"location-feature-flag": {
  "key": "location-feature-flag",
  "on": true,
  "targets": [{"values": ["LOC123"], "variation": 0}],
  "fallthrough": { "variation": 1 },
  "variations": [false, true]
}
```

**What the sample app does:**  
- `GET /locations/LOC123/booleantest` → flag evaluates to `false` → returns `400 feature_disabled_by_killswitch`.  
- `GET /locations/ANYOTHER/booleantest` → flag evaluates to `true` → calls the mock API and returns its response.

> **Real‑world use:** You would replace the mock API with a real downstream service (e.g. inventory, pricing). The kill‑switch lets you instantly cut traffic to that service if it becomes unhealthy or if a new feature causes issues.

### Flag 2: `log-levels` (JSON)

**Purpose:** Dynamically control the application’s logging verbosity.  
**Type:** JSON.  
**Variations (array of strings):**

| Variation | Value (array of allowed log levels) | Effective minimum level |
|-----------|--------------------------------------|--------------------------|
| 0 | `["DEBUG","INFO","WARN","ERROR"]` | `DEBUG` |
| 1 | `["INFO","WARN","ERROR"]`           | `INFO` |
| 2 | `["WARN","ERROR"]`                  | `WARN` |
| 3 | `["ERROR"]`                         | `ERROR` |

**Targeting:**  
- For most users, use variation 3 (`["ERROR"]`) in production to keep logs quiet.  
- For a specific user (e.g. `context.key = "debug-user"`), target variation 0 to get `DEBUG` logs.  
- You can also target by custom attributes (e.g. `store = "NYC"`).

**In `flags.json`:**
```json
"log-levels": {
  "key": "log-levels",
  "on": true,
  "variations": [
    ["DEBUG","INFO","WARN","ERROR"],
    ["INFO","WARN","ERROR"],
    ["WARN","ERROR"],
    ["ERROR"]
  ],
  "fallthrough": { "variation": 3 }
}
```

**How the code uses it:**  
`LogLevelManager` polls this flag, reads the array, and computes the **minimum** `slog` level present (e.g. `DEBUG` is lowest). It then configures a `slog.Handler` that only enables log records at or above that level.  
Change the flag in LaunchDarkly → within 30 seconds (poll interval) the log level changes **without restarting** the service.

---

## 5. What to Do in Production vs Development

| Aspect | **Development / Testing** | **Production** |
|--------|---------------------------|----------------|
| **LaunchDarkly connection** | Use `offline: true` or omit SDK key. The library reads `flags.json` – no network calls, deterministic. | Set `offline: false` and provide a valid `sdkKey`. The SDK will stream flag updates from LD. |
| **Cache** | Keep `cacheEnabled: true` – it speeds up repeated requests and helps test caching logic. | **Keep enabled**. The cache dramatically reduces SDK calls. Tune `expireAfterWriteMinute` (e.g. 1–5 minutes) based on how quickly you need flag changes to propagate. |
| **Redis persistent store** | Usually disabled (unless you are testing Redis integration). | Enable if you have **many service instances** – Redis shares flag data and reduces load on LaunchDarkly’s servers. Set `local-ttl-seconds` (e.g. 30) to avoid stale data. |
| **Dynamic log levels** | Use `log-levels` flag in `flags.json`. Set fallthrough to `0` (`["DEBUG"]`) for verbose logs during development. | In production, set fallthrough to `3` (`["ERROR"]`). Use targeting to temporarily enable `DEBUG` for a specific user or instance when debugging an issue. |
| **HTTP client (retry/circuit breaker)** | Enable with low thresholds (e.g. max attempts 2, short timeouts) to test resilience. | Enable with production‑tuned values (e.g. 3 attempts, exponential backoff, circuit breaker with 50% failure threshold, 30s open state). |
| **flags.json** | Keep in the repository as a reference and for offline tests. | **Do not use** in production. Production should connect to live LaunchDarkly. |
| **Logging verbosity** | Run with `DEBUG` logs enabled (via `log-levels` or directly in code). | Run with `INFO` or `ERROR` by default. Use the dynamic flag to increase verbosity only when needed. |
| **Graceful shutdown** | Not critical, but good to test. | **Essential**. Always call `ldClient.Close()` and `logMgr.Stop()` to avoid leaking goroutines and connections. |

### Quick Configuration Checklist

**Development `config.yaml`:**
```yaml
ldkillswitch:
  offline: true
  cacheEnabled: true
  expireAfterWriteMinute: 1
redis:
  enabled: false
```

**Production `config.yaml`:**
```yaml
ldkillswitch:
  offline: false
  sdkKey: "prod-sdk-key-123"
  cacheEnabled: true
  expireAfterWriteMinute: 2
redis:
  enabled: true
  url: "redis://prod-redis:6379"
  local-ttl-seconds: 30
```

---

## 6. Putting It All Together – The Big Picture

The library and sample app together demonstrate a **modern microservice pattern**:

1. **All external decisions** (feature toggles, kill‑switches, log levels) are controlled by LaunchDarkly.
2. **Performance** is ensured by a multi‑layer caching strategy:  
   - In‑memory cache (`go-cache`) → LD SDK → (optional) Redis → LaunchDarkly servers.
3. **Resilience** is built‑in:  
   - If LD is down, the cache serves stale values (still better than falling back to default on every request).  
   - The circuit breaker protects downstream dependencies.
4. **Observability** is dynamic – you can change log levels without restarting, which is crucial for debugging production incidents.
5. **Developer experience** is excellent – offline mode means you can develop and test without any external dependencies.

**In short:** This codebase is a blueprint for integrating LaunchDarkly into Go services in a way that is fast, reliable, and easy to operate. Use the `LDKillSwitch` library as‑is, and follow the `SampleApp` patterns for your own business logic.



The key is to set this up as a **JSON flag** in the LaunchDarkly dashboard, not a standard string flag. A JSON flag allows you to store a list, like `["DEBUG","INFO"]`, which your application can then interpret to control its log level.

Here is the step-by-step guide to create and configure the flag.

### 🏷️ Step 1: Create a New JSON Flag

1.  **Navigate to Feature Flags**: In your LaunchDarkly dashboard, go to the **Feature flags** page and click the **Create flag** button.
2.  **Choose Flag Type**: In the creation modal, select the **Custom** flag template. Then, for the **Flag type**, select **JSON**. This is the most important step, as it allows you to store arrays and objects.
3.  **Name Your Flag**: Enter a descriptive name, like "Application Log Level". A unique **Flag key** (e.g., `log-levels`) will be generated. You'll use this exact key in your code (`"log-levels"`).

### 📝 Step 2: Define the JSON Variations

Next, you will define the possible values the flag can return. These must be **valid JSON**. Your application is expecting an array of strings, so your variations should look like this:

*   **Variation 0**: `["DEBUG", "INFO", "WARN", "ERROR"]` (Most verbose, for development)
*   **Variation 1**: `["INFO", "WARN", "ERROR"]` (Standard for production)
*   **Variation 2**: `["WARN", "ERROR"]` (For a quieter, high-volume environment)
*   **Variation 3**: `["ERROR"]` (Least verbose, for critical systems only)

You can add variations by clicking the **+ Add variation** button and entering each JSON array. You can also give each variation a friendly name for easy identification, like "Dev (All Levels)" or "Prod (Info+)".

### 🎯 Step 3: Configure Targeting Rules

Targeting rules determine which variation a user or service receives. This is where you set the flag's behavior for different environments.

1.  **Turn the Flag On**: On the flag's **Targeting** tab, toggle the status to **On**.
2.  **Set the Fallthrough Rule**: The **Fallthrough** (or default rule) is the value served to contexts that don't match any specific targets. Set this to your standard production level, e.g., Variation 1: `["INFO", "WARN", "ERROR"]`.
3.  **Add Specific Targets (Optional)**: You can target specific contexts to override the fallthrough rule. For example:
    *   Target your development service's context key (e.g., `sample-app-instance`) and serve Variation 0 to enable `DEBUG` logs.
    *   Create a rule that targets a specific user email and serves Variation 2 for quieter logs.

### 💻 Step 4: Verify in Your Code

Once the flag is saved, your running application will automatically detect the change (based on your `pollInterval` of 30 seconds) and adjust its log level without needing a restart.

*   **No changes to your Go code are needed**. The `log_manager.go` you already have is designed to work with this exact JSON flag structure. It will fetch the JSON array, parse it, and set the `slog` level to the most verbose level in the list (e.g., `DEBUG` if present).

By following these steps, you'll have a flexible, dynamic log level system that can be controlled centrally from LaunchDarkly.
