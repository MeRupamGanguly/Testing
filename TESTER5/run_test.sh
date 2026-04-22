#!/bin/bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}=== Running full functional tests with Docker ===${NC}"

# 1. Start PostgreSQL container
echo -e "${GREEN}Starting PostgreSQL container...${NC}"
docker rm -f test-postgres 2>/dev/null || true
docker run -d --name test-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=secret \
  -e POSTGRES_DB=ecommerce \
  -p 5432:5432 \
  postgres:15-alpine

# Wait for PostgreSQL
sleep 5

# 2. Run migrations
echo -e "${GREEN}Running database migrations...${NC}"
docker exec -i test-postgres psql -U postgres -d ecommerce < sampleApp/schema.sql

# 3. Build test image
echo -e "${GREEN}Building test container...${NC}"
docker build -t ecommerce-test -f - . <<EOF
FROM golang:1.26.1-alpine
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /app/order-service ./sampleApp/cmd
EOF

# 4. Run unit tests
echo -e "${GREEN}Running unit tests...${NC}"
docker run --rm ecommerce-test go test -v -cover ./...

# 5. Run the service with PostgreSQL
# Setting the working directory to /app/sampleApp/cmd ensures 
# that the binary finds ../../config.yaml at /app/config.yaml
echo -e "${GREEN}Starting order-service...${NC}"
docker rm -f order-service 2>/dev/null || true
docker run -d --name order-service \
  --network host \
  -w /app/sampleApp/cmd \
  -v $(pwd)/config.yaml:/app/config.yaml:Z \
  ecommerce-test /app/order-service

sleep 3

# 6. API end‑to‑end tests
echo -e "${GREEN}Running API tests...${NC}"
RESPONSE=$(curl -s -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"customer_id":"test123","items":[{"product_id":"p1","quantity":2,"price":"19.99","currency":"USD"}]}')
ORDER_ID=$(echo $RESPONSE | jq -r '.id')

if [ "$ORDER_ID" != "null" ] && [ -n "$ORDER_ID" ]; then
    echo -e "${GREEN}✓ Order created: $ORDER_ID${NC}"
else
    echo -e "${RED}✗ Create order failed${NC}"
    docker logs order-service
    exit 1
fi

# GET order using query param
curl -s "http://localhost:8080/orders?id=$ORDER_ID" | jq . > /dev/null && echo -e "${GREEN}✓ Get order OK${NC}" || exit 1

# Confirm order using query param
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://localhost:8080/orders/confirm?id=$ORDER_ID")
[ "$HTTP_CODE" = "204" ] && echo -e "${GREEN}✓ Confirm OK${NC}" || exit 1

# Cancel order (should fail)
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "http://localhost:8080/orders/cancel?id=$ORDER_ID")
[ "$HTTP_CODE" = "400" ] && echo -e "${GREEN}✓ Cancel rejection OK${NC}" || exit 1

# Cleanup
echo -e "${GREEN}Cleaning up...${NC}"
docker stop order-service test-postgres
docker rm order-service test-postgres

echo -e "${GREEN}=== All tests passed successfully ===${NC}"
