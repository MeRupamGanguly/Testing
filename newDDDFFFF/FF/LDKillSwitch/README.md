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
The Logic Flow:

Application Request: Service layer requests a feature flag evaluation.

Cache Check: The library computes a unique CacheKey based on the Flag Key + User Context.

Cache Hit: Returns the value immediately from memory (0.1ms ~ 0.5ms).

Cache Miss: Calls the LaunchDarkly SDK, updates the local cache, and returns the fresh value.
## Requirements
Go Version: 1.18 or higher.

LaunchDarkly SDK: github.com/launchdarkly/go-server-sdk/v7.

Dependencies:

github.com/patrickmn/go-cache (For in-memory caching).

github.com/launchdarkly/go-server-sdk-redis-go-redis (Optional, for Redis support).


## Core Component:
 Cache :- acts as a middle-layer cache between the application and the LaunchDarkly SDK. It uses Guava for Java and go-cache for go to store boolean flag results in memory to reduce SDK overhead. Cache HIT and MISS handled from here.

 Service:- It has only Job query the IsFeatureEnabled by calling Cache. A clean API entry point that abstracts the complexity of the cache and SDK from your main application.

 Config:-  it holds the sdkKey and an offlineMode toggle. and CacheEnabled, ExpireAfterWriteMinute, MaximumSize etc Variables.

 ID_Client_config:-  It checks if an SDK key is provided; if not, it uses a fallback then It create a launchdarkly go Server client conection. And also return the Client. Handles the initialization of the LDClient. Manages the switch between Offline Mode and Live Redis Mode.

For local development, set offline: true. The library will automatically look for a flags.json file in your root directory to evaluate rules without needing an internet connection.
```json
{
  "flags": {
    "location-feature-flag": {
      "key": "location-feature-flag",
      "on": true,
      "variations": [false, true],
      "fallthrough": { "variation": 1 }
    }
  }
}
```


1. The Call Chain
When you hit the /locations/LOC123/... endpoint, the following happens:

Your Controller (controller.go) builds an ldcontext.

Your Service (serviceapp.go) calls s.featureFlagService.IsFeatureEnabled.

Your Library (cache.go) checks its local go-cache.

The SDK Call: If it's a cache miss, your code executes f.ldClient.BoolVariation(flagKey, ctx, defaultValue). This is the official SDK call.

2. Where the SDK gets its data
Even though you are in "Offline Mode," you are still using the full power of the SDK's evaluation engine.

Instead of calling the LaunchDarkly servers over the internet, the SDK is directed by your ld_client.go configuration to use ldfiledata.DataSource().FilePaths("flags.json").

The SDK reads your JSON, parses the complex targeting rules (like the one we just added for LOC123), and performs the logic math locally.


SampleApp: Feature Flag & Kill-Switch Integration Demo
This SampleApp is a production-grade demonstration of how to integrate the LDKillSwitch library into a Go-based microservice architecture. It simulates a high-throughput e-commerce location service that utilizes LaunchDarkly for real-time feature management, Circuit Breakers for fault tolerance, and In-Memory Caching for performance.

🏗️ Architecture Overview
The application follows a clean layered architecture to decouple business logic from infrastructure concerns:

Controller Layer: Handles incoming HTTP requests via Gin, extracts metadata (like Location IDs), and builds the ldcontext.

Service Layer: Orchestrates the business logic. It queries the LDKillSwitch service to decide if a downstream call should be executed or blocked.

Client Layer: A resilient Web Client wrapper featuring automated retries and a Circuit Breaker pattern using gobreaker.

LDKillSwitch Integration: Acts as the gatekeeper, providing cached flag evaluations to prevent unnecessary overhead.

🚀 Key Features
Contextual Kill-Switch: Demonstrates blocking specific API calls based on locationId or custom attributes (system, channel, store).

Resilient Web Client: Includes a robust client with:

Retries: Configurable exponential backoff.

Circuit Breaker: Prevents cascading failures by "tripping" when failure thresholds are met.

Containerized Testing: Ready for Podman/Docker with a multi-stage build for a tiny production footprint.

Offline First: Fully functional without a live LaunchDarkly connection using flags.json.


Using the Test SuiteThe easiest way to see the app in action is using the provided Podman test script:Bash./rusn_test.sh
Manual ExecutionEnsure you have flags.json and config.yaml in your root.Run the main entry point:Bashgo run ./SampleApp/cmd/main.go
🧪 API EndpointsEndpointDescriptionGET /locations/:id/booleantestA standard check. If the flag is false for this ID, it returns a 400 Kill-Switch error.GET /locations/:id/booleanattributestestTests complex targeting using query params (sourceSystem, store).GET /api/mock/locations/:idThe internal mock destination. Only reached if the Kill-Switch allows it.📦 DependenciesWeb Framework: github.com/gin-gonic/ginResilience: github.com/sony/gobreakerConfig: gopkg.in/yaml.v3Kill-Switch: LDKillSwitch Library (internal)
