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

# Wait duration for fsnotify to trigger reload
WAIT_SECONDS=195

# Function to change fallthrough variation for log-levels
set_log_level_variation() {
    local variation=$1
    echo -e "\n${BLUE}Changing /app/flags.json: set log-levels fallthrough variation to $variation${NC}"
    podman exec $CONTAINER_NAME sh -c "sed '/\"log-levels\": {/,/}/ s/\"fallthrough\": { \"variation\": [0-9]* }/\"fallthrough\": { \"variation\": $variation }/' /app/flags.json > /tmp/flags.json && cat /tmp/flags.json > /app/flags.json"
    echo -e "${GREEN}/app/flags.json updated.${NC}"
    echo -e "${BLUE}Waiting $WAIT_SECONDS seconds for fsnotify to trigger reloader...${NC}"
    for i in $(seq 1 $WAIT_SECONDS); do
        printf "\rWaiting... %d/%d seconds" $i $WAIT_SECONDS
        sleep 1
    done
    echo ""
}

# Step 1 – initial state: log level = ERROR (variation 3) → no app DEBUG logs
echo -e "\n${BLUE}[Step 1] Initial state – expecting NO slog DEBUG logs (fallthrough = 3 / ERROR)${NC}"
curl -s "$BASE_URL/locations/ANY/booleantest"
sleep 2

if check_app_debug_logs; then
    echo -e "${RED}FAIL: slog DEBUG logs appeared when they should not (log level = ERROR)${NC}"
    podman stop $CONTAINER_NAME > /dev/null
    exit 1
else
    echo -e "${GREEN}PASS: No slog DEBUG logs – initial log level is ERROR${NC}"
fi

# Define test cases: (variation, expect_debug_logs, description)
test_cases=(
    "4 true   'variation 4 = [\"DEBUG\"] – should show DEBUG logs'"
    "2 false  'variation 2 = [\"WARN\",\"ERROR\"] – should NOT show DEBUG logs'"
    "0 true   'variation 0 = [\"DEBUG\",\"INFO\",\"WARN\",\"ERROR\"] – should show DEBUG logs'"
)

# Iterate through test cases
for tc in "${test_cases[@]}"; do
    read -r var expect desc <<< "$tc"
    echo -e "\n${BLUE}[Step] Testing $desc${NC}"

    set_log_level_variation "$var"

    # Trigger a request to generate logs
    curl -s "$BASE_URL/locations/ANY/booleantest"
    sleep 2

    if [ "$expect" = "true" ]; then
        if check_app_debug_logs; then
            echo -e "${GREEN}SUCCESS: DEBUG logs appeared as expected for variation $var${NC}"
        else
            echo -e "${RED}FAIL: DEBUG logs missing for variation $var (expected true)${NC}"
            podman stop $CONTAINER_NAME > /dev/null
            exit 1
        fi
    else
        if check_app_debug_logs; then
            echo -e "${RED}FAIL: DEBUG logs appeared for variation $var (expected false)${NC}"
            podman stop $CONTAINER_NAME > /dev/null
            exit 1
        else
            echo -e "${GREEN}PASS: No DEBUG logs as expected for variation $var${NC}"
        fi
    fi
done

# Cleanup
echo -e "\n${BLUE}=== Test completed. Cleaning up... ===${NC}"
podman stop $CONTAINER_NAME > /dev/null
podman rm $CONTAINER_NAME > /dev/null
echo -e "${GREEN}Done.${NC}"
