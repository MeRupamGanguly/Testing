#!/bin/bash

IMAGE_NAME="killswitch-app"
CONTAINER_NAME="killswitch-test-instance"
PORT=8080
BASE_URL="http://localhost:$PORT"

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== LaunchDarkly KillSwitch – Dynamic Log Level Test ===${NC}"

# Cleanup
podman rm -f $CONTAINER_NAME 2>/dev/null

# Build
echo -e "${BLUE}Building container...${NC}"
podman build -t $IMAGE_NAME .
if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed${NC}"
    exit 1
fi

# Run
echo -e "${BLUE}Starting container...${NC}"
podman run -d --name $CONTAINER_NAME -p $PORT:8080 $IMAGE_NAME
sleep 3

# Helper: look for slog DEBUG lines only (exclude Gin)
check_app_debug_logs() {
    echo -e "\n${BLUE}Checking for slog DEBUG logs (not Gin) in container...${NC}"
    DEBUG_LOGS=$(podman logs $CONTAINER_NAME 2>&1 | grep '"level":"DEBUG"' | grep -v "GIN-debug" | head -5)
    if [ -n "$DEBUG_LOGS" ]; then
        echo -e "${GREEN}Application DEBUG logs found:${NC}"
        echo "$DEBUG_LOGS"
        return 0
    else
        echo -e "${RED}No application DEBUG logs found.${NC}"
        return 1
    fi
}

# Step 1 – initial state: log level = ERROR → no app DEBUG logs
echo -e "\n${BLUE}[Step 1] Initial state – expecting NO slog DEBUG logs (fallthrough = ERROR)${NC}"
curl -s "$BASE_URL/locations/ANY/booleantest" > /dev/null
sleep 2

if check_app_debug_logs; then
    echo -e "${RED}FAIL: slog DEBUG logs appeared when they should not (log level = ERROR)${NC}"
    podman stop $CONTAINER_NAME > /dev/null
    exit 1
else
    echo -e "${GREEN}PASS: No slog DEBUG logs – initial log level is ERROR${NC}"
fi

# Step 2 – change flags.json inside container to set log-levels fallthrough to DEBUG (variation 0)
echo -e "\n${BLUE}[Step 2] Changing /app/flags.json: set log-levels fallthrough variation to 0 (DEBUG)${NC}"
podman exec $CONTAINER_NAME sh -c "cat > /app/flags.json << 'EOF'
{
  \"flags\": {
    \"location-feature-flag\": {
      \"key\": \"location-feature-flag\",
      \"on\": true,
      \"targets\": [{\"values\": [\"LOC123\"], \"variation\": 0}],
      \"fallthrough\": { \"variation\": 1 },
      \"variations\": [false, true]
    },
    \"log-levels\": {
      \"key\": \"log-levels\",
      \"on\": true,
      \"variations\": [
        [\"DEBUG\",\"INFO\",\"WARN\",\"ERROR\"],
        [\"INFO\",\"WARN\",\"ERROR\"],
        [\"WARN\",\"ERROR\"],
        [\"ERROR\"]
      ],
      \"fallthrough\": { \"variation\": 0 }
    }
  }
}
EOF"
echo -e "${GREEN}/app/flags.json updated.${NC}"

# Step 3 – wait for cache TTL (2 min) + poll interval (30 sec) + margin
WAIT_SECONDS=150
echo -e "\n${BLUE}[Step 3] Waiting $WAIT_SECONDS seconds for cache to expire and poller to run...${NC}"
for i in $(seq 1 $WAIT_SECONDS); do
    printf "\rWaiting... %d/%d seconds" $i $WAIT_SECONDS
    sleep 1
done
echo ""

# Step 4 – trigger requests and expect slog DEBUG logs
echo -e "\n${BLUE}[Step 4] Triggering requests – expecting slog DEBUG logs to appear${NC}"
curl -s "$BASE_URL/locations/ANY/booleantest" > /dev/null
sleep 2

if check_app_debug_logs; then
    echo -e "${GREEN}SUCCESS: Dynamic log level changed from ERROR to DEBUG without restart!${NC}"
else
    echo -e "${RED}FAIL: slog DEBUG logs still missing after waiting. Check cache TTL or poll interval.${NC}"
    podman stop $CONTAINER_NAME > /dev/null
    exit 1
fi

# Step 5 – revert to ERROR and verify DEBUG logs disappear
echo -e "\n${BLUE}[Step 5] Changing /app/flags.json back to ERROR (fallthrough variation 3)${NC}"
podman exec $CONTAINER_NAME sh -c "sed -i 's/\"fallthrough\": { \"variation\": 0 }/\"fallthrough\": { \"variation\": 3 }/' /app/flags.json"
echo "Waiting another $WAIT_SECONDS seconds..."
sleep $WAIT_SECONDS

curl -s "$BASE_URL/locations/ANY/booleantest" > /dev/null
sleep 2
if check_app_debug_logs; then
    echo -e "${RED}FAIL: slog DEBUG logs still present after reverting to ERROR${NC}"
    exit 1
else
    echo -e "${GREEN}PASS: Log level successfully reverted to ERROR (no slog DEBUG logs)${NC}"
fi

# Cleanup
echo -e "\n${BLUE}=== Test completed. Cleaning up... ===${NC}"
podman stop $CONTAINER_NAME > /dev/null
podman rm $CONTAINER_NAME > /dev/null
echo -e "${GREEN}Done.${NC}"
