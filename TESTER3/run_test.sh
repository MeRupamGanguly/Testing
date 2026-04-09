#!/bin/bash
NETWORK="tester-net"
APP_NAME="webapp"
REDIS_NAME="redis"

echo "🚀 Starting System Test..."

# 1. Force Cleanup (Added -f to force removal of existing containers)
podman rm -f $APP_NAME $REDIS_NAME >/dev/null 2>&1

# 2. Setup Network
podman network inspect $NETWORK >/dev/null 2>&1 || podman network create $NETWORK

# 3. Start Redis
podman run -d --name $REDIS_NAME --network $NETWORK -p 6379:6379 redis:alpine

# 4. Build & Run
podman build -t webapp-img .
podman run -d --name $APP_NAME \
  --network $NETWORK \
  -p 8080:8080 \
  -e LD_OFFLINE=true \
  -e REDIS_URL="redis://$REDIS_NAME:6379" \
  webapp-img

echo "⏳ Waiting for app startup..."
sleep 4

# --- 5. FEATURE TESTING ---

echo -e "\n--- [TEST 1: Core Lib / FeatureFlagCache] ---"
# This worked!
curl -i "http://localhost:8080/locations/123/booleantest"

echo -e "\n--- [TEST 2: Parent App / Controller + Logic] ---"
# Added more params to match the Java Controller logic
curl -i "http://localhost:8080/locations/123/booleanattributestest?sourceSystem=mobile&sourceChannel=app&store=45"

echo -e "\n--- [TEST 3: Internal Logs] ---"
# Check if we hit the Cache or the Circuit Breaker
podman logs $APP_NAME | grep -E "Cache|Request|Response|Circuit"

echo -e "\n--- [TEST 4: Performance / Cache Check] ---"
echo "First Call (Should be a Cache Miss):"
time curl -s "http://localhost:8080/locations/999/booleantest"
echo -e "\nSecond Call (Should be a Cache Hit):"
time curl -s "http://localhost:8080/locations/999/booleantest"
echo -e "\n🏁 Done."
