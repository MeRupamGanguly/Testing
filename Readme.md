# LaunchDarkly Kill-Switch Library

This library provides a high-performance, cached wrapper for the LaunchDarkly Go SDK. It is designed to minimize network overhead and latency by implementing an in-memory caching layer and supporting persistent data stores like Redis.

The library acts as an intermediary between your application and LaunchDarkly. Instead of evaluating a feature flag through the SDK on every request—which can add latency—the library checks an internal cache first.

Logic Flow: Application → Service Layer → Cache Layer (check memory) → LaunchDarkly SDK (if miss).

Correct Contextual Caching: Unlike basic implementations, this library caches values based on a unique combination of the Flag Key and the User Context, ensuring data integrity across different flags for the same user.


Key Features
Performance Optimization: Uses an in-memory TTL (Time-To-Live) cache to provide near-instant flag evaluations.

Redis Integration: Supports Redis as a persistent data store, allowing the SDK to remain synchronized across multiple service instances without hitting LaunchDarkly's servers repeatedly.

Offline Mode: Ability to run the SDK in offline mode for local development or testing environments where an internet connection is unavailable.

Thread-Safe: Built on go-cache, ensuring safe concurrent access for high-traffic Go routines.

Error Resiliency: Gracefully handles SDK errors by returning a user-defined default value if evaluation fails.


Requirements
Go Version: 1.18 or higher.

LaunchDarkly SDK: github.com/launchdarkly/go-server-sdk/v7.

Dependencies:

github.com/patrickmn/go-cache (For in-memory caching).

github.com/launchdarkly/go-server-sdk-redis-go-redis (Optional, for Redis support).


How to Use This Library
1. Define Configuration
Set up your properties for the SDK and the cache.

```go
props := config.LaunchDarklyProperties{
    SdkKey:      "your-sdk-key",
    OfflineMode: false,
}

cacheProps := config.FeatureFlagCacheProperties{
    CacheEnabled:           true,
    ExpireAfterWriteMinute: 5 * time.Minute,
}
```
2. Initialize the Client and Service
Initialize the LD Client (with optional Redis) and wrap it in the Cache and Service layers.

``` go
// Initialize the core SDK client
ldClient := config.NewLDClient(props, true, "localhost:6379")

// Initialize the Cache wrapper
ffCache := cache.NewFeatureFlagCache(ldClient, cacheProps)

// Initialize the Service layer
ffService := service.NewFeatureFlagService(ffCache)
```
3. Evaluate a Flag
Use the service to check if a feature is enabled for a specific context.

```go
userContext := ldcontext.New("user-123")
isEnabled := ffService.IsFeatureEnabled("my-new-feature", userContext, false)

if isEnabled {
    // Execute feature logic
}
```


----

Java Files Analysis
t1.java (FeatureFlagCache): This is a Spring @Component that acts as a middle-layer cache between the application and the LaunchDarkly SDK. It uses Guava Cache to store boolean flag results in memory to reduce SDK overhead. However, it contains a logic error: the cache key is only the user ID, meaning it cannot distinguish between two different flags for the same user.

t2.java / t3.java (FeatureFlagCacheAutoConfiguration): These identical files provide Spring Boot auto-configuration. They ensure the FeatureFlagCache bean is only created if an LDClient is already present in the Spring context.

c2.java (LaunchDarklyProperties): A simple POJO mapped to the launchdarkly configuration prefix. it holds the sdkKey and an offlineMode toggle.

cc1.java (LaunchDarklyConfig): This class initializes the LDClient bean. It checks if an SDK key is provided; if not, it uses a fallback @Value or initializes the client based on the offlineMode property.

s1.java (FeatureFlagService): The public-facing service that other parts of the application call. It wraps the cache and provides the isFeatureEnabled method.

Equivalent Go Files and Improvements
Your Go code follows the same structure but is functionally more robust.

1. Configuration (config.go)
Java Equivalent: c2.java.

Purpose: Defines the LaunchDarklyProperties and FeatureFlagCacheProperties structs. While Java uses Spring's @ConfigurationProperties, Go uses standard structs that are typically populated via a library like Viper or environment variables.

2. Client Initialization (ld_client.go)
Java Equivalent: cc1.java.

Purpose: Initializes the LDClient.

Improvement: Unlike the Java version, this Go code includes Redis DataStore support. If useRedis is true, the Go client will use Redis as a persistent store for flag rules, which is significantly more scalable than the purely in-memory Java approach.

3. Caching Layer (cache.go)
Java Equivalent: t1.java.

Purpose: Uses the go-cache library (a Go equivalent to Guava) to store results.

Improvement (Fix): The Go version fixes the major bug in the Java code by creating a cache key that combines the flag key and the user key: fmt.Sprintf("%s:%s", flagKey, ctx.Key()). This allows multiple different flags to be cached correctly for the same user.

4. Service Layer (service.go)
Java Equivalent: s1.java.

Purpose: Provides the IsFeatureEnabled method, which acts as the entry point for the application. It correctly passes the defaultValue through to the cache, whereas the Java service was hardcoding false as the default.


The Go Approach (Explicit Wiring)
Go does not have a built-in "Auto-configuration" or "Annotation" system that scans your project and connects pieces automatically.

Manual Injection: In Go, you are expected to manually connect your components in your main.go or a wire-up function.

The Go equivalent is logic, not a file: The logic found in t2.java (passing the client and properties to the cache constructor) is already handled in your Go code by the NewFeatureFlagCache function in cache.go.

Simplicity: Instead of a separate configuration file, a Go developer would typically write something like this in their main entry point:

Go
// This is the manual equivalent of what t2.java does automatically
client := config.NewLDClient(props, true, "redis://...")
cache := cache.NewFeatureFlagCache(client, cacheProps)
service := service.NewFeatureFlagService(cache)
3. Summary of Why Go Skips These Files
No Reflection/Annotations: Go avoids the "magic" of checking if a bean exists at runtime using annotations.

Constructor Injection: In your Go code, NewFeatureFlagService and NewFeatureFlagCache explicitly require their dependencies as arguments, making a "Conditional" configuration file unnecessary.

Compile-time Safety: Go prefers that you see exactly how the service gets the cache and how the cache gets the client, rather than having a background process (Spring) do it for you.
