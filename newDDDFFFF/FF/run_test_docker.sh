#!/bin/bash

IMAGE_NAME="killswitch-app"
CONTAINER_NAME="killswitch-test-instance"
PORT=8080
BASE_URL="http://localhost:$PORT"

LOG_FILE="test_logs_$(date +%Y%m%d_%H%M%S).txt"
echo -e "Logs will be saved to: $LOG_FILE"

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== LaunchDarkly KillSwitch – Dynamic Log Level Test (Single Step) ===${NC}"

# Cleanup
docker rm -f $CONTAINER_NAME 2>/dev/null
pkill -f "docker logs -f $CONTAINER_NAME" 2>/dev/null

# Build
echo -e "${BLUE}Building container...${NC}"
docker build -t $IMAGE_NAME .
if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed${NC}"
    exit 1
fi

# Run container
echo -e "${BLUE}Starting container...${NC}"
docker run -d --name $CONTAINER_NAME -p $PORT:8080 $IMAGE_NAME

# Wait for app to be healthy
echo -n "Waiting for app to be ready"
for i in {1..30}; do
    if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e "\n${RED}App failed to start within 30 seconds${NC}"
        docker logs $CONTAINER_NAME
        docker rm -f $CONTAINER_NAME
        exit 1
    fi
done

# Give the app a moment to fully initialise
echo -e "${BLUE}Waiting 5 seconds for initialisation...${NC}"
sleep 5

# Start log streaming to both terminal and file
docker logs -f $CONTAINER_NAME 2>&1 | tee "$LOG_FILE" &
LOG_PID=$!

cleanup() {
    echo -e "\n${BLUE}Cleaning up...${NC}"
    kill $LOG_PID 2>/dev/null
    docker stop $CONTAINER_NAME > /dev/null
    docker rm $CONTAINER_NAME > /dev/null
    echo -e "${GREEN}Logs saved to $LOG_FILE${NC}"
}
trap cleanup EXIT

# Helper to check for application DEBUG logs (exclude internal log-level manager logs)
check_app_debug_logs() {
    # Wait a bit for logs to be written
    sleep 2
    # Use grep -a to treat file as text
    DEBUG_LOGS=$(grep -a '"level":"DEBUG"' "$LOG_FILE" | \
                 grep -v "GIN-debug" | \
                 grep -v "log-levels" | \
                 grep -v "GetJSONFlag" | \
                 grep -v "Cache lookup (JSON)" | \
                 tail -10)
    if [ -n "$DEBUG_LOGS" ]; then
        echo -e "${GREEN}Application DEBUG logs found:${NC}"
        echo "$DEBUG_LOGS"
        return 0
    else
        echo -e "${RED}No application DEBUG logs found.${NC}"
        return 1
    fi
}

# Clear the log file before the test
> "$LOG_FILE"

# Step 1: Initial variation = 0 (["ERROR"]) – ensure no DEBUG logs initially
echo -e "\n${BLUE}[Initial] Variation 0 = [\"ERROR\"] – expecting NO DEBUG logs${NC}"
curl -s "$BASE_URL/locations/ANY/booleantest" > /dev/null
if check_app_debug_logs; then
    echo -e "${RED}FAIL: Unexpected DEBUG logs found before changing flag${NC}"
    exit 1
else
    echo -e "${GREEN}PASS: No DEBUG logs initially${NC}"
fi

# Clear logs before the actual test
> "$LOG_FILE"

# Step 2: Change fallthrough variation to 1 (["DEBUG","INFO"])
echo -e "\n${BLUE}[Test] Setting log-levels fallthrough variation to 1 (should enable DEBUG)${NC}"
docker exec $CONTAINER_NAME sh -c \
    "sed -i -E '/\"log-levels\":/,/}/ s/\"fallthrough\": *\{ *\"variation\": *[0-9]+ *\}/\"fallthrough\": { \"variation\": 1 }/' /app/flags.json"
NEW_VAL=$(docker exec $CONTAINER_NAME sh -c "grep -A5 '\"log-levels\"' /app/flags.json | grep fallthrough")
echo -e "${GREEN}Updated: $NEW_VAL${NC}"

# Wait for the file watcher (or poller) to pick up the change
echo -e "${BLUE}Waiting 5 seconds for flag reload...${NC}"
sleep 5

# Make a request to trigger logs
curl -s "$BASE_URL/locations/ANY/booleantest" > /dev/null

# Check for DEBUG logs
if check_app_debug_logs; then
    echo -e "${GREEN}SUCCESS: DEBUG logs appeared as expected${NC}"
else
    echo -e "${RED}FAIL: Expected DEBUG logs but none found${NC}"
    exit 1
fi

echo -e "\n${BLUE}=== Test passed ===${NC}"
