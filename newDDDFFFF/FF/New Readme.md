# LaunchDarkly Kill‑Switch Library

A production‑grade Go library that wraps the LaunchDarkly server SDK with an in‑memory caching layer, offline mode, Redis persistence, and dynamic log‑level control. Designed for high‑throughput microservices where low latency and resilience are critical.

---

## Introduction

This repository contains **two cooperating modules**:

- **`LDKillSwitch`** – a reusable Go library that enhances the official LaunchDarkly SDK. It adds a TTL‑based memory cache (using `go-cache`), optional Redis as a persistent data store, a clean service interface, and a dynamic log‑level manager that polls a JSON flag to adjust `slog` verbosity at runtime.

- **`SampleApp`** – a fully functional demonstration application (simulated location service) that shows how to integrate the library into a real microservice. It implements a kill‑switch pattern, a resilient HTTP client with retries and circuit breaker, and the dynamic log‑level feature.

Together they provide a **blueprint** for using LaunchDarkly in Go services that are fast, reliable, and easy to operate.

---

## Why This Library?

The official LaunchDarkly Go SDK is powerful, but in high‑throughput environments every call to `BoolVariation` or `StringVariation` adds measurable latency – even though the SDK streams updates efficiently. This library solves three main problems:

- **Performance** – A local cache returns flag values in microseconds instead of milliseconds. Many services evaluate the same flag for the same user context repeatedly (e.g., on every HTTP request). Caching eliminates redundant SDK calls.
- **Resilience** – If LaunchDarkly is temporarily unreachable, the cache still holds the last known value. Your service does not fall back to a default on every request during a network blip.
- **Offline development** – Developers can work without an internet connection or a LaunchDarkly account. The library reads a local `flags.json` file and the SDK evaluates targeting rules locally. All tests become deterministic and fast.

Additionally, the built‑in **dynamic log‑level manager** allows you to change logging verbosity without restarting the service – invaluable for debugging production incidents.

---

## How LaunchDarkly Works (Briefly)

LaunchDarkly is a feature management platform. You define feature flags in the dashboard, set targeting rules (e.g., “show this flag only to users with email ending in @example.com”), and the SDK evaluates those rules for each context (user, device, etc.). The SDK streams flag changes in near real‑time, so updated values become available without redeploying.

The official SDK already caches flag rules, but it still performs an evaluation – which includes rule parsing and data structure lookups – for every call. This library places an **additional cache layer** in front of the SDK, storing the *result* of the evaluation for a given flag key and context. When the same flag+context is requested again, the library returns the cached value instantly.

---

## Core Concepts

**Cache Key Design**  
The library builds a unique cache key from three parts: the flag type (`bool`, `str`, or `json`), the flag key, and the LaunchDarkly context key (e.g., `"bool:my-flag:user-123"`). This ensures that different flags or different users never interfere with each other’s cached values.

**Time‑To‑Live (TTL) and Cleanup**  
The underlying `go-cache` library provides an expiration time per entry. You configure `ExpireAfterWriteMinute` in the cache properties. After that period, the entry is removed and the next evaluation will hit the SDK again. A cleanup interval runs automatically (twice the TTL) to remove expired entries and free memory.

**Context‑Aware Evaluation**  
All flag evaluation methods accept a `ldcontext.Context`. This context can contain custom attributes like `sourceSystem`, `store`, or `channel`. LaunchDarkly uses these attributes in targeting rules. The library caches the *evaluation result* for the exact context, so two different contexts with the same flag key will have separate cache entries.

**Offline Mode**  
When `offline` is set to `true` in the configuration (or when no SDK key is provided), the library configures the LaunchDarkly SDK to use a local file data source (`ldfiledata`). The SDK reads `flags.json` and evaluates targeting rules locally. No network calls are made to LaunchDarkly servers. The in‑memory cache still works, and dynamic log levels also function using the local flag definitions.

**Redis Persistent Data Store**  
For production environments with many service instances, you can enable Redis as a persistent data store for the LaunchDarkly SDK. The SDK stores flag rules in Redis, and all instances share the same data. This reduces the number of network calls to LaunchDarkly’s servers and keeps flag data synchronised across instances. You can also set a local TTL for the in‑memory cache that sits in front of Redis.

**Dynamic Log‑Level Manager**  
The `LogLevelManager` polls a JSON flag (e.g., `"log-levels"`) at a configurable interval (default 30 seconds). The flag returns an array of strings, such as `["DEBUG","INFO","WARN","ERROR"]`. The manager computes the most verbose level present in the array (e.g., `DEBUG`) and configures the global `slog` handler to only emit records at or above that level. This allows you to change log verbosity instantly by updating the flag in LaunchDarkly (or the local `flags.json` in offline mode).

---

## File‑by‑File Explanation (LDKillSwitch Library)

**`cache.go`**  
Implements the `FeatureFlagCache` struct. It holds a `go-cache` instance and a reference to the LaunchDarkly client. Methods like `GetBooleanFlagValue`, `GetStringFlagValue`, and `GetJSONFlagValue` first compute a cache key from the flag key and context, then check the cache. On a miss, they call the corresponding `Variation` method on the LD client, store the result, and return it. The `GetJSONFlagValue` method also includes a helper to convert arbitrary Go default values into `ldvalue.Value` by marshalling to JSON. A separate `GetJSONFlagNoCache` bypasses the cache entirely – used by the log manager to get fresh values without polluting the cache.

**`cache_test.go`**  
Unit tests for the cache layer. Uses LaunchDarkly’s `testhelpers/ldtestdata` to create a mock data source. Tests cover cache hit, cache miss, cache disabled, fallback to default value, and correct key isolation between different flags and contexts.

**`config.go`**  
Simple configuration structs: `LaunchDarklyProperties` (SDK key and offline mode) and `FeatureFlagCacheProperties` (cache enabled flag and expiration duration). These are used when creating the LD client and the cache.

**`ld_client.go`**  
Contains the `NewLDClient` factory function. It inspects the properties: if offline mode is enabled or the SDK key is empty, it builds a client that reads from a local `flags.json` file using `ldfiledata.DataSource()`. Otherwise, it creates a live client. If Redis is enabled, it configures a persistent data store with `ldcomponents.PersistentDataStore(redisStore).CacheSeconds(localTtlSecond)`. The function returns a ready‑to‑use `*ldclient.LDClient` or an error.

**`service.go`**  
Provides `FeatureFlagService` – a thin wrapper around `FeatureFlagCache`. This is the intended entry point for application code. It exposes `IsFeatureEnabled`, `GetStringFlag`, `GetJSONFlag`, and `GetJSONFlagNoCache`. The service layer abstracts away the cache implementation details and makes it easy to mock or replace in tests.

**`service_test.go`**  
Tests the service layer using the same mock data source. Verifies that `IsFeatureEnabled` returns the expected value for known flags and falls back to the default for unknown flags.

**`log_manager.go`**  
Implements the `LogLevelManager`. It starts a goroutine that polls a JSON flag at a given interval. The flag’s value is expected to be an array of strings representing log levels. The manager calculates the minimum (most verbose) level in the array and stores it atomically. It also provides a custom `slog.Handler` that delegates to a base handler but overrides the `Enabled` method to check the current dynamic level. This handler can be set as the default `slog` logger.

**`go.mod`**  
Lists all dependencies: LaunchDarkly Go SDK v7, `go-cache`, Redis integration, and test helpers.

**`flags.json`**  
Example flag definitions for offline mode. Contains a boolean flag `location-feature-flag` with a target for `LOC123` returning `false`, and a JSON flag `log-levels` with several variations (e.g., `["ERROR"]`, `["DEBUG","INFO"]`). This file is read by the SDK when offline mode is active.

**Original `README.md` and `Notes.md`**  
Documentation files explaining the architecture, usage, and testing strategies.

---

## API Reference – What Each Method Does and How It Works Internally

The library exposes two main layers: the `FeatureFlagCache` (low‑level) and the `FeatureFlagService` (recommended for application use). All methods are safe for concurrent use.

### `GetBooleanFlagValue(flagKey, ctx, defaultValue) bool`

**What it does**  
Evaluates a boolean feature flag for the given LaunchDarkly context. Returns the flag’s value if successful, otherwise returns the provided `defaultValue`.

**Internal flow**  
1. Constructs a cache key: `"bool:" + flagKey + ":" + ctx.Key()`.
2. If caching is enabled, checks the in‑memory cache. On hit, returns the stored boolean immediately (microsecond latency).
3. On cache miss (or cache disabled), calls `ldClient.BoolVariation(flagKey, ctx, defaultValue)`. This invokes the LaunchDarkly SDK to evaluate the flag rules.
4. Stores the result in the cache with the configured TTL, then returns it.

### `GetStringFlagValue(flagKey, ctx, defaultValue) string`

**What it does**  
Evaluates a string feature flag. Returns the string value from LaunchDarkly or the default.

**Internal flow**  
Identical to `GetBooleanFlagValue`, but uses a cache key prefixed with `"str:"` and calls `ldClient.StringVariation`. The SDK returns a string, which is stored and retrieved as‑is.

### `GetJSONFlagValue(flagKey, ctx, defaultValue) interface{}`

**What it does**  
Evaluates a JSON feature flag. The flag can return any JSON‑serializable type: a slice, a map, a primitive, or a nested structure. The method returns the value as a generic `interface{}` (e.g., `[]interface{}`, `map[string]interface{}`).

**Internal flow**  
1. Uses cache key `"json:" + flagKey + ":" + ctx.Key()`.
2. On cache hit, returns the stored value.
3. On miss, it first converts the Go `defaultValue` into an `ldvalue.Value` by marshalling it to JSON and unmarshalling into an `ldvalue.Value`. This is necessary because the LaunchDarkly SDK’s `JSONVariation` expects an `ldvalue.Value` as the default.
4. Calls `ldClient.JSONVariation(flagKey, ctx, defaultVal)`.
5. Converts the returned `ldvalue.Value` back to a raw Go type using `.AsRaw()`.
6. Stores the raw value and returns it.

### `GetJSONFlagNoCache(flagKey, ctx, defaultValue) interface{}`

**What it does**  
Evaluates a JSON flag but **bypasses the cache entirely**. Always calls the LaunchDarkly SDK directly. This is used by the `LogLevelManager` to get fresh flag values without polluting the cache with frequently‑changing data.

**Internal flow**  
Skips the cache lookup and storage steps. Converts the default value to `ldvalue.Value`, calls `ldClient.JSONVariation`, and returns `.AsRaw()`. No cache interaction occurs.

### `IsFeatureEnabled(flagKey, ctx, defaultValue) bool` (via `FeatureFlagService`)

**What it does**  
A convenience wrapper around `GetBooleanFlagValue`. Intended to make code more readable when using flags as kill‑switches or feature gates.

**Internal flow**  
Simply delegates to `cache.GetBooleanFlagValue` with the same parameters. The service layer adds no extra logic – it exists to decouple your application from the concrete cache implementation.

### `GetStringFlag(flagKey, ctx, defaultValue) string` and `GetJSONFlag(...)`

**What they do**  
Same as the cache methods but exposed through the service layer. They provide a consistent interface if you later decide to replace the caching strategy or add additional behaviour (e.g., metrics, logging). Currently they are thin wrappers.

---

## How the APIs Work with and Without an SDK Key

The behaviour of every API changes depending on whether a valid LaunchDarkly SDK key is provided and whether offline mode is enabled. The library abstracts this difference completely – your application code does not need to know which mode is active.

### When an SDK Key is Present (Live Mode)

The library initialises a real LaunchDarkly client using `ldclient.MakeCustomClient`. This client establishes a streaming connection to LaunchDarkly’s servers and receives flag rule updates in near real‑time.

**API behaviour in live mode**  
- On the first call for a specific `(flagKey, context)` pair, the cache is empty. The method calls the LaunchDarkly SDK, which evaluates the flag using the latest rules (streamed from the cloud). The result is stored in the local in‑memory cache.
- Subsequent calls for the same pair return the cached value instantly, **without any network or SDK evaluation**.
- When the cache TTL expires, the next call triggers a fresh SDK evaluation. The new result may reflect flag changes that happened in the LaunchDarkly dashboard in the meantime.
- If Redis is enabled as a persistent data store, the SDK itself may serve flag rules from Redis instead of fetching them from LaunchDarkly every time. The library’s own cache still sits on top, further reducing latency.
- The `GetJSONFlagNoCache` method always bypasses the cache, so it always calls the SDK (and therefore the streaming/Redis data source). This is used by the log‑level manager to poll for changes.

**What happens when LaunchDarkly servers are unreachable?**  
If the SDK cannot connect (network issue, outage), it falls back to the last known flag rules stored locally or in Redis. The library’s cache also holds previously evaluated results. Subsequent calls will return cached values (if they exist) or the SDK’s last known evaluation. If neither is available, the SDK returns the default value. The library never panics or blocks.

### When No SDK Key is Present (Offline Mode)

This happens when `offline: true` is set in the configuration, or when the provided SDK key is an empty string. The library configures the LaunchDarkly SDK to use a local file data source: `ldfiledata.DataSource().FilePaths("flags.json")`. The SDK reads the JSON file once at startup and then watches for file changes (using `ldfilewatch.WatchFiles`). No network connections are made to LaunchDarkly.

**API behaviour in offline mode**  
- The cache works exactly as in live mode. The only difference is the source of truth for flag evaluations.
- On a cache miss, the SDK evaluates the flag using the rules defined in `flags.json`. Because there is no network, this evaluation is instantaneous (microseconds).
- If you edit `flags.json` while the application is running, the file watcher detects the change and reloads the flag rules. The next cache miss will see the updated rules.
- The `GetJSONFlagNoCache` method still bypasses the cache, so it always reads the current state of `flags.json` via the SDK.
- All methods honour targeting rules defined in the JSON file. For example, if `flags.json` contains a target that says `"LOC123"` should receive variation 0 (false), then `IsFeatureEnabled("my-flag", ctxForLOC123, true)` will return `false`.

**Key takeaway**  
Your application code does not need `if offlineMode` checks. The same API calls work identically in both modes. This makes local development and integration testing seamless – you can run the service without any external dependencies and be confident that it will behave the same way in production (except for the real‑time update speed, which is faster in live mode).

---

## How to Test the Library

### Unit Tests

Run all tests inside the `LDKillSwitch` directory:

```bash
cd LDKillSwitch
go test -v ./...
```

The tests use `ldtestdata` to simulate LaunchDarkly without any network calls. They verify cache hit/miss behaviour, cache disabling, fallback to default values, and correct isolation between different flags and contexts.

### Integration / Manual Testing

You can test the library in isolation by writing a small `main` package that imports it. However, the provided `SampleApp` offers a complete test harness.

**Testing with offline mode (no SDK key)**  
Set `offline: true` in `config.yaml` (or leave the SDK key empty). Run the sample app with `go run ./SampleApp/cmd/main.go`. Send HTTP requests to endpoints like `/locations/ANY/booleantest`. The flag evaluation will come from `flags.json`. Edit `flags.json` while the app is running – the file watcher will reload the changes, and subsequent cache misses will use the new rules.

**Testing with live mode (valid SDK key)**  
Obtain a LaunchDarkly SDK key from your account. Set `offline: false` and provide the key in `config.yaml`. Create the two flags (`location-feature-flag` and `log-levels`) in the LaunchDarkly dashboard. Run the sample app – it will connect to LaunchDarkly. Change flag values in the dashboard; the sample app will reflect the changes within a few seconds (plus the cache TTL).

**Testing the dynamic log level**  
The scripts `run_test.sh` (Podman) and `run_test_docker.sh` (Docker) automate a complete test of the log‑level feature. They start the container, verify that no `DEBUG` logs appear initially (when the flag returns `["ERROR"]`), then modify the `flags.json` file inside the container to change the fallthrough variation to `["DEBUG","INFO"]`, wait for the file watcher to reload, and finally check that `DEBUG` logs appear. This demonstrates that dynamic log levels work in offline mode. For live mode, you would change the flag in the LaunchDarkly dashboard instead of editing the file.

---

## Production vs Development Configuration

**For development and testing**  
Set `offline: true` or omit the SDK key. The library will read `flags.json` from the filesystem. Keep `cacheEnabled: true` with a short TTL (e.g., 1 minute) to test caching behaviour. Disable Redis unless you are explicitly testing the Redis integration. Use a verbose log‑level fallthrough in `flags.json` (e.g., `["DEBUG","INFO"]`) during active development.

**For production**  
Set `offline: false` and provide a valid `sdkKey`. Enable the cache with a TTL that balances freshness and performance – typically 2 to 5 minutes. Consider enabling Redis as a persistent data store if you run many service instances; set `local-ttl-seconds` to a moderate value (e.g., 30). Set the fallthrough of the `log-levels` flag to `["ERROR"]` or `["WARN","ERROR"]` to keep logs quiet by default. Use targeting rules in LaunchDarkly to temporarily enable `DEBUG` logs for specific contexts when debugging.

**Never use the `flags.json` file in production** – it should only be used for offline development and testing. Production must connect to live LaunchDarkly to receive real‑time flag updates.

---

## Requirements

- Go 1.18 or higher
- LaunchDarkly account (only for live mode)
- Redis (optional, for persistent data store)

Main dependencies (managed by Go modules):
- `github.com/launchdarkly/go-server-sdk/v7`
- `github.com/patrickmn/go-cache`
- `github.com/launchdarkly/go-server-sdk-redis-go-redis` (optional)

---
The code you've provided isn't just a simple "Hello World" app; it is a sophisticated boilerplate designed to demonstrate **Feature Management** and **Dynamic Configuration** using LaunchDarkly.

The reason there are multiple APIs and layers is to show how you can control application behavior (like blocking a request or changing log levels) in real-time without redeploying your code.

-----

### 1\. The HTTP API Endpoints (`controller.go`)

These are the entry points for external users.

| API Endpoint | What it does | What it needs |
| :--- | :--- | :--- |
| `GET /locations/:locationId/booleanattributestest` | **The "Rich Context" Test.** It checks if a feature is enabled based on specific user attributes (where they are coming from, what store they are in). | **Path:** `locationId`<br>**Query Params:** `sourceSystem`, `sourceChannel`, `store`. |
| `GET /locations/:locationId/booleantest` | **The "Simple" Test.** It checks the same feature flag but uses a bare-minimum context (just the ID). Useful for testing default behaviors. | **Path:** `locationId`. |
| `GET /api/mock/locations/:locationId` | **The Mock Target.** This isn't a real business API; it acts as the "Destination" for the `WebClientService` to call so you can see the full flow locally. | **Path:** `locationId`. |

-----

### 2\. The Logic Layer (`serviceapp.go`)

This is the **"Kill Switch"** layer. This isn't an API exposed to the web, but an internal service API used by the controller.

  * **What it does:** Before it makes a network call to fetch data, it asks LaunchDarkly: *"Is the feature 'location-feature-flag' enabled for this specific user/context?"*
  * **Why it's required:** It demonstrates a **Circuit Breaker** or **Kill Switch** pattern. If a downstream service is failing, you can flip a switch in LaunchDarkly, and this code will block the request immediately at line 40, saving resources and preventing errors.

-----

### 3\. The Dynamic Logging API (`log_manager.go`)

This represents a **Configuration API**. It doesn't have a URL; instead, it "polls" LaunchDarkly in the background.

  * **What it does:** It watches a JSON flag in LaunchDarkly (e.g., a list like `["DEBUG", "INFO"]`).
  * **Why it's required:** In production, you usually run at `INFO` or `ERROR` level to save disk space. If a bug happens, you can update the LaunchDarkly flag to `DEBUG`. This manager will see the change and immediately start showing those `slog.Debug` lines from your controller without you having to restart the server.

-----

### 4\. The Configuration Layer (`ld_client.go`)

This is the **Infrastructure API**.

  * **What it does:** It manages the connection to LaunchDarkly. It's smart enough to switch between "Offline Mode" (reading from a local `flags.json` file) and "Live Mode" (using a real SDK key and optionally Redis for high-performance caching).
  * **Why it's required:** It ensures that your application is resilient. If the internet goes down, the app can fall back to local files or a Redis cache so the APIs don't break.

-----

### Summary: Why this many?

1.  **To separate concerns:** The `Controller` handles the web request, the `Service` handles the "Should I do this?" logic, and the `LogManager` handles the "How much should I talk?" logic.
2.  **To test different Scenarios:** By having both `booleanattributestest` and `booleantest`, you can test if your LaunchDarkly rules work correctly when attributes are missing versus when they are present.
3.  **To simulate a real environment:** The `MockTargetAPI` allows you to run this whole project on your laptop and see a "Successful" response without needing a real database or secondary microservice.

**Essentially: It's a lab environment to prove that you can control every aspect of your app (traffic, logic, and logging) from a remote dashboard.**


The code you've provided isn't just a simple "Hello World" app; it is a sophisticated boilerplate designed to demonstrate **Feature Management** and **Dynamic Configuration** using LaunchDarkly.

The reason there are multiple APIs and layers is to show how you can control application behavior (like blocking a request or changing log levels) in real-time without redeploying your code.

-----

### 1\. The HTTP API Endpoints (`controller.go`)

These are the entry points for external users.

| API Endpoint | What it does | What it needs |
| :--- | :--- | :--- |
| `GET /locations/:locationId/booleanattributestest` | **The "Rich Context" Test.** It checks if a feature is enabled based on specific user attributes (where they are coming from, what store they are in). | **Path:** `locationId`<br>**Query Params:** `sourceSystem`, `sourceChannel`, `store`. |
| `GET /locations/:locationId/booleantest` | **The "Simple" Test.** It checks the same feature flag but uses a bare-minimum context (just the ID). Useful for testing default behaviors. | **Path:** `locationId`. |
| `GET /api/mock/locations/:locationId` | **The Mock Target.** This isn't a real business API; it acts as the "Destination" for the `WebClientService` to call so you can see the full flow locally. | **Path:** `locationId`. |

-----

### 2\. The Logic Layer (`serviceapp.go`)

This is the **"Kill Switch"** layer. This isn't an API exposed to the web, but an internal service API used by the controller.

  * **What it does:** Before it makes a network call to fetch data, it asks LaunchDarkly: *"Is the feature 'location-feature-flag' enabled for this specific user/context?"*
  * **Why it's required:** It demonstrates a **Circuit Breaker** or **Kill Switch** pattern. If a downstream service is failing, you can flip a switch in LaunchDarkly, and this code will block the request immediately at line 40, saving resources and preventing errors.

-----

### 3\. The Dynamic Logging API (`log_manager.go`)

This represents a **Configuration API**. It doesn't have a URL; instead, it "polls" LaunchDarkly in the background.

  * **What it does:** It watches a JSON flag in LaunchDarkly (e.g., a list like `["DEBUG", "INFO"]`).
  * **Why it's required:** In production, you usually run at `INFO` or `ERROR` level to save disk space. If a bug happens, you can update the LaunchDarkly flag to `DEBUG`. This manager will see the change and immediately start showing those `slog.Debug` lines from your controller without you having to restart the server.

-----

### 4\. The Configuration Layer (`ld_client.go`)

This is the **Infrastructure API**.

  * **What it does:** It manages the connection to LaunchDarkly. It's smart enough to switch between "Offline Mode" (reading from a local `flags.json` file) and "Live Mode" (using a real SDK key and optionally Redis for high-performance caching).
  * **Why it's required:** It ensures that your application is resilient. If the internet goes down, the app can fall back to local files or a Redis cache so the APIs don't break.

-----
The `cache.go` file acts as the "Engine Room" for your feature flag system. While LaunchDarkly has its own internal caching, this layer provides a local, high-performance in-memory cache to ensure your application doesn't experience latency when checking flags thousands of times per second.

Each method serves a specific functional purpose for performance, type safety, or real-time responsiveness:

### 1. The Setup: `NewFeatureFlagCache`
* **Purpose**: This is the constructor that initializes the `go-cache` instance.
* **Why it's required**: It defines how long a flag value stays "fresh" in memory (`expirationTime`) and how often the system purges old data (`cleanupInterval`). This ensures you aren't wasting RAM on stale flag data.

### 2. Type-Specific Performance: `GetBooleanFlagValue` & `GetStringFlagValue`
* **Purpose**: These provide fast lookups for simple flag types (True/False or Text).
* **Why they are required**:
    * **Type Safety**: LaunchDarkly’s Go SDK is strongly typed; you must call the specific method (`BoolVariation` or `StringVariation`) that matches the flag type in the dashboard.
    * **Efficiency**: By checking the local `flagCache` first, you avoid the overhead of the full LaunchDarkly evaluation logic for every single request.

### 3. Complex Data Handling: `GetJSONFlagValue` & `convertToLDValue`
* **Purpose**: These handle complex objects like configuration maps or lists of strings.
* **Why they are required**: 
    * **The Compatibility Bridge**: LaunchDarkly requires complex types to be passed as `ldvalue.Value`. `convertToLDValue` is a helper that translates standard Go interfaces into a format the SDK understands.
    * **Structure**: `GetJSONFlagValue` allows you to store entire "feature configurations" (not just on/off switches) in memory for instant access.

### 4. The Real-Time Exception: `GetJSONFlagNoCache`
* **Purpose**: This method explicitly ignores the local cache and asks the LaunchDarkly client for the absolute latest value.
* **Why it's required**:
    * **Immediate Response**: Some features cannot wait for a cache to expire. 
    * **Example Case**: Your `LogLevelManager` uses this method. If you are debugging a live issue and change the log level from `ERROR` to `DEBUG` in the dashboard, you want that change to happen **immediately** across your system without waiting for a 5-minute cache TTL.

---

### Summary Table: When to use which?

| Method | Use Case | Performance |
| :--- | :--- | :--- |
| **`GetBoolean...`** | Standard Kill Switches (On/Off) | **Fastest** (In-memory) |
| **`GetString...`** | Dynamic URLs or Labels | **Fastest** (In-memory) |
| **`GetJSON...`** | Complex Config Objects | **Fast** (In-memory) |
| **`GetJSONNoCache`** | System Overrides (like Log Levels) | **Slower** (SDK Eval) |
### Summary: Why this many?

1.  **To separate concerns:** The `Controller` handles the web request, the `Service` handles the "Should I do this?" logic, and the `LogManager` handles the "How much should I talk?" logic.
2.  **To test different Scenarios:** By having both `booleanattributestest` and `booleantest`, you can test if your LaunchDarkly rules work correctly when attributes are missing versus when they are present.
3.  **To simulate a real environment:** The `MockTargetAPI` allows you to run this whole project on your laptop and see a "Successful" response without needing a real database or secondary microservice.

**Essentially: It's a lab environment to prove that you can control every aspect of your app (traffic, logic, and logging) from a remote dashboard.**
## Conclusion

The `LDKillSwitch` library gives you a fast, resilient, and developer‑friendly way to integrate LaunchDarkly into Go microservices. By adding an in‑memory cache, offline mode, Redis support, and dynamic log‑level control, it removes the friction that often comes with feature flag systems. The `SampleApp` demonstrates real‑world patterns that you can adapt to your own services.

Use this library to reduce latency, increase reliability, and gain instant observability control – all while keeping your code clean and testable.





The three core cache APIs – `GetBooleanFlagValue`, `GetStringFlagValue`, and `GetJSONFlagValue` – each follow the same internal pattern but with different type handling. Here’s exactly how data flows from your controller down to the LaunchDarkly core library and back.

## Internal call chain for any of the three APIs

```
Controller → Service → FeatureFlagCache → go-cache → LaunchDarkly SDK → (optional Redis / LD servers)
```

### Step‑by‑step breakdown

**1. Controller calls service layer**  
In `controller.go`, you call `webClientService.Get()`, which internally calls `featureFlagService.IsFeatureEnabled()` (for a boolean flag). That service method is a thin wrapper around the cache.

**2. Service calls the corresponding cache method**  
`IsFeatureEnabled` calls `cache.GetBooleanFlagValue()`. Similarly, `GetStringFlag` calls `GetStringFlagValue`, and `GetJSONFlag` calls `GetJSONFlagValue`.

**3. Cache method constructs a type‑prefixed key**  
Example for boolean: `cacheKey := fmt.Sprintf("bool:%s:%s", flagKey, ctx.Key())`  
This key uniquely identifies the flag value for that flag key and context key.

**4. Cache checks local in‑memory store (`go-cache`)**  
- If `cacheEnabled` is true and the key exists and hasn’t expired, the cached value is returned immediately.  
- For boolean/string, the value is cast to the correct type (`val.(bool)`). For JSON, the stored `interface{}` is returned as‑is.

**5. On cache miss, the cache calls the LaunchDarkly SDK**  
- `GetBooleanFlagValue` → `f.ldClient.BoolVariation(flagKey, ctx, defaultValue)`  
- `GetStringFlagValue` → `f.ldClient.StringVariation(...)`  
- `GetJSONFlagValue` → first converts the Go `defaultValue` to an `ldvalue.Value` via `convertToLDValue()` (JSON marshal/unmarshal), then calls `f.ldClient.JSONVariation(...)`

**6. The LaunchDarkly SDK evaluates the flag**  
- If in **live mode** (SDK key present), the SDK uses its internal rule cache (populated by a streaming connection to LaunchDarkly servers) to evaluate the flag against the provided context. No network call happens per evaluation – the rules are already local.  
- If in **offline mode** (no SDK key or `offline: true`), the SDK reads flag rules from the local `flags.json` file (via `ldfiledata`) and evaluates them locally.

**7. The SDK returns the evaluated value**  
- `BoolVariation` returns a `bool`  
- `StringVariation` returns a `string`  
- `JSONVariation` returns an `ldvalue.Value` (which the cache then converts to a raw Go type via `.AsRaw()`)

**8. The cache stores the fresh value**  
`f.flagCache.Set(cacheKey, freshValue, cache.DefaultExpiration)` – using the TTL configured in `NewFeatureFlagCache`.

**9. The value returns up the chain**  
Cache → service → controller → HTTP response.

## How data goes to the core library (LaunchDarkly)

- **Live mode** – The LaunchDarkly SDK maintains a persistent streaming connection to `https://app.launchdarkly.com`. Flag rule updates are pushed to the SDK in real time. The SDK stores them in memory (and optionally in Redis if configured). When `BoolVariation` is called, the SDK evaluates the rule locally using those in‑memory rules – **no per‑call network latency**.
- **Offline mode** – The SDK reads `flags.json` at startup and watches for file changes. All evaluations are purely local; no data ever leaves your process.

The `FeatureFlagCache` sits **in front** of the SDK, caching the *result* of the evaluation. This avoids calling the SDK at all for repeated identical flag+context requests.

## Why three separate APIs?

Each cache method is tailored to the SDK method it calls and the type it returns. They cannot be combined because:
- The SDK has no generic `Variation` method.
- Cache keys need type prefixes (`bool:`, `str:`, `json:`) to prevent collisions.
- JSON flags require an extra conversion step (`convertToLDValue`) that would be wasted on bool/string flags.

So the three APIs mirror the three SDK evaluation methods, with caching added transparently.
