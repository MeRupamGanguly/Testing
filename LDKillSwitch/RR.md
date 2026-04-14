This project demonstrates a Go-based microservice architecture integrating a custom LaunchDarkly Kill-Switch library. It features a robust web client equipped with Circuit Breaker patterns, Exponential Backoff Retries, and Feature Flag validation via LaunchDarkly.

🚀 Core Features

Feature Flag Guarding: Validates flags before making downstream API calls to prevent unnecessary traffic if a service is "killed".

Resilience Patterns:

Circuit Breaker: Uses gobreaker to stop cascading failures when downstream services are unhealthy.

Retry Mechanism: Implements configurable exponential backoff for transient error recovery.

Multi-Level Configuration: Supports global and per-service overrides for timeouts, URLs, and retry logic via YAML.


Contextual Routing: Uses LaunchDarkly contexts (LocationID, Source System, etc.) to evaluate flag variations per request.

🚦 API Endpoints
1. Get Location (With Attributes)
GET /locations/:locationId/booleanattributestest?sourceSystem=web&store=123

Evaluates the location-feature-flag using the provided query parameters and path ID.

If enabled, forwards the request to the Mock Target API.

2. Get Location (Basic)
GET /locations/:locationId/booleantest

Evaluates the flag using only the locationId as the context key.

3. Mock Target
GET /api/mock/locations/:locationId

Simulates the downstream microservice response.


🧪 Logic Flow: The Kill-Switch in Action

Request: Controller receives an HTTP request.


Context: An ldcontext is built using request parameters.


Gatekeeper: WebClientService calls IsFeatureEnabled.


If False: Returns feature_disabled_by_killswitch (400 Bad Request).

If True: Proceeds to the next step.

Execution: WebClient executes the request through the Circuit Breaker and Retry wrappers.


Response: The final data is returned to the user.




Build and Run
Build Image:

Bash
docker build -t sample-app .
Run Container:

Bash
docker run -p 8080:8080 sample-app
Note: Ensure your config.yaml and flags.json are present in the project root before building, as they are copied into the final image.
