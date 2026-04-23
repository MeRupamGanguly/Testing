#!/bin/bash
set -e

# ------------------------------------------------------------
# Configuration
# ------------------------------------------------------------
PROJECT_DIR=$(pwd)
IMAGE_NAME="crosscutting-api"
CONTAINER_NAME="crosscutting-api-test"
NETWORK_NAME="crosscutting-test-net"
REDIS_CONTAINER="crosscutting-redis"
JAEGER_CONTAINER="crosscutting-jaeger"
PORT=8080
JWT_SECRET="super-secret-jwt-key-for-testing"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

# ------------------------------------------------------------
# Pre-cleanup
# ------------------------------------------------------------
echo -e "${YELLOW}Cleaning up any previous test resources...${NC}"
podman stop "$CONTAINER_NAME" "$REDIS_CONTAINER" "$JAEGER_CONTAINER" 2>/dev/null || true
podman rm "$CONTAINER_NAME" "$REDIS_CONTAINER" "$JAEGER_CONTAINER" 2>/dev/null || true
podman network rm -f "$NETWORK_NAME" 2>/dev/null || true
podman network prune -f 2>/dev/null || true

# ------------------------------------------------------------
# JWT generation (no newlines, URL-safe base64)
# ------------------------------------------------------------
b64enc() {
    # Encode without padding or newlines
    echo -n "$1" | base64 | tr -d '=\n' | tr '/+' '_-'
}

generate_jwt() {
    local sub=$1
    local role=$2
    local header='{"alg":"HS256","typ":"JWT"}'
    local payload="{\"sub\":\"$sub\",\"role\":\"$role\",\"exp\":1999999999}"
    local header_b64=$(b64enc "$header")
    local payload_b64=$(b64enc "$payload")
    local signature=$(echo -n "$header_b64.$payload_b64" | openssl dgst -sha256 -hmac "$JWT_SECRET" -binary | base64 | tr -d '=\n' | tr '/+' '_-')
    echo "$header_b64.$payload_b64.$signature"
}

TOKEN_CUSTOMER=$(generate_jwt "customer-123" "customer")
TOKEN_ADMIN=$(generate_jwt "admin-1" "admin")

# ------------------------------------------------------------
# Cleanup on exit
# ------------------------------------------------------------
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    podman stop "$CONTAINER_NAME" "$REDIS_CONTAINER" "$JAEGER_CONTAINER" 2>/dev/null || true
    podman rm "$CONTAINER_NAME" "$REDIS_CONTAINER" "$JAEGER_CONTAINER" 2>/dev/null || true
    podman network rm -f "$NETWORK_NAME" 2>/dev/null || true
}
trap cleanup EXIT

# ------------------------------------------------------------
# Check required tools
# ------------------------------------------------------------
if ! command -v jq &> /dev/null; then
    echo -e "${YELLOW}jq not found. Install with: sudo dnf install jq${NC}"
    echo -e "${YELLOW}Tracing test will be skipped.${NC}"
    JQ_AVAILABLE=false
else
    JQ_AVAILABLE=true
fi

# ------------------------------------------------------------
# Setup network and dependencies
# ------------------------------------------------------------
echo -e "${YELLOW}Creating Podman network...${NC}"
podman network create "$NETWORK_NAME"

echo -e "${YELLOW}Starting Redis...${NC}"
podman run -d --name "$REDIS_CONTAINER" --network "$NETWORK_NAME" \
    docker.io/library/redis:7-alpine

echo -e "${YELLOW}Starting Jaeger...${NC}"
podman run -d --name "$JAEGER_CONTAINER" --network "$NETWORK_NAME" \
    -e COLLECTOR_OTLP_ENABLED=true \
    -p 16686:16686 \
    docker.io/jaegertracing/all-in-one:latest

# ------------------------------------------------------------
# Build API container
# ------------------------------------------------------------
echo -e "${YELLOW}Building API image...${NC}"
podman build -t "$IMAGE_NAME" -f - . <<'EOF'
FROM docker.io/library/golang:1.26.1-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/main.go

FROM docker.io/library/alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
EOF

# ------------------------------------------------------------
# Run API container
# ------------------------------------------------------------
echo -e "${YELLOW}Starting API container...${NC}"
podman run -d --name "$CONTAINER_NAME" --network "$NETWORK_NAME" \
    -p "$PORT":8080 \
    -e REDIS_ADDR="$REDIS_CONTAINER:6379" \
    -e JWT_SECRET="$JWT_SECRET" \
    -e OTEL_EXPORTER_OTLP_ENDPOINT="http://$JAEGER_CONTAINER:4317" \
    "$IMAGE_NAME"

# Wait for readiness
echo -e "${YELLOW}Waiting for API to become ready...${NC}"
for i in {1..15}; do
    if curl -s "http://localhost:$PORT/health" >/dev/null 2>&1; then
        echo -e "${GREEN}API is up!${NC}"
        break
    fi
    sleep 1
    if [ $i -eq 15 ]; then
        echo -e "${RED}API failed to start${NC}"
        exit 1
    fi
done

# ------------------------------------------------------------
# Test helper (now using arrays for headers)
# ------------------------------------------------------------
test_endpoint() {
    local desc="$1"
    local method="$2"
    local url="$3"
    local expected_code="$4"
    shift 4
    # All remaining arguments are passed directly to curl (e.g., -H "Header: value")
    local curl_args=("$@")

    echo -n "Testing $desc ... "
    # Execute curl with arguments, capture status and body
    response=$(curl -s -w "\n%{http_code}" -X "$method" "$url" "${curl_args[@]}" 2>&1)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -eq "$expected_code" ]; then
        echo -e "${GREEN}PASS${NC} (HTTP $http_code)"
    else
        echo -e "${RED}FAIL${NC} (expected $expected_code, got $http_code)"
        echo "Response body: $body"
        exit 1
    fi
}

BASE_URL="http://localhost:$PORT"

# ------------------------------------------------------------
# Run tests
# ------------------------------------------------------------
echo -e "\n${YELLOW}Running tests...${NC}\n"

# 1. Health
test_endpoint "health" "GET" "$BASE_URL/health" 200

# 2. Metrics
test_endpoint "metrics" "GET" "$BASE_URL/metrics" 200

# 3. Profile without token → 401
test_endpoint "profile without token" "GET" "$BASE_URL/api/v1/profile" 401

# 4. Profile with customer token → 200
test_endpoint "profile with customer token" "GET" "$BASE_URL/api/v1/profile" 200 \
    -H "Authorization: Bearer $TOKEN_CUSTOMER"

# 5. Invalid order payload → 400
test_endpoint "create order invalid payload" "POST" "$BASE_URL/api/v1/orders" 400 \
    -H "Authorization: Bearer $TOKEN_CUSTOMER" \
    -H "Content-Type: application/json" \
    -d '{"product_sku":"bad","quantity":0,"price":-1}'

# 6. Valid order → 201
test_endpoint "create order valid payload" "POST" "$BASE_URL/api/v1/orders" 201 \
    -H "Authorization: Bearer $TOKEN_CUSTOMER" \
    -H "Content-Type: application/json" \
    -d '{"product_sku":"SKU12345","quantity":2,"price":29.99}'

# 7. Admin endpoint with customer → 403
test_endpoint "admin endpoint with customer" "GET" "$BASE_URL/api/v1/admin/users" 403 \
    -H "Authorization: Bearer $TOKEN_CUSTOMER"

# 8. Admin endpoint with admin → 200
test_endpoint "admin endpoint with admin" "GET" "$BASE_URL/api/v1/admin/users" 200 \
    -H "Authorization: Bearer $TOKEN_ADMIN"

# 9. Rate limiting (expect 429 after many requests)
echo -n "Testing rate limiting (101 rapid requests) ... "
RATE_LIMIT_HIT=0
for i in {1..101}; do
    code=$(curl -s -o /dev/null -w "%{http_code}" -H "Authorization: Bearer $TOKEN_CUSTOMER" "$BASE_URL/api/v1/profile")
    if [ "$code" -eq 429 ]; then
        RATE_LIMIT_HIT=1
        break
    fi
done
if [ "$RATE_LIMIT_HIT" -eq 1 ]; then
    echo -e "${GREEN}PASS${NC} (429 received)"
else
    echo -e "${RED}FAIL${NC} (no rate limit triggered after 101 requests)"
    exit 1
fi

# 10. Tracing check
if [ "$JQ_AVAILABLE" = true ]; then
    echo -n "Checking Jaeger for traces... "
    sleep 3
    TRACES=$(curl -s "http://localhost:16686/api/traces?service=ecommerce-api" | jq '.data | length')
    if [ "$TRACES" -gt 0 ]; then
        echo -e "${GREEN}PASS${NC} (found $TRACES traces)"
    else
        echo -e "${YELLOW}WARN${NC} (no traces found)"
    fi
else
    echo -e "${YELLOW}Skipping tracing test (jq not installed)${NC}"
fi

echo -e "\n${GREEN}All critical tests passed!${NC}\n"
