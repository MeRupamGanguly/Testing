#!/bin/bash
set -e

# Configuration
NETWORK_NAME="batch-test-net"
DB_CONTAINER="batch-db-test"
APP_CONTAINER="batch-app-test"
IMAGE_NAME="batch-scheduler:test"
TOTAL_TASKS=140

echo " Starting Full Functional Test (Docker Edition)..."

# 1. Cleanup old environments
echo "Cleaning up old environments..."
docker rm -f $DB_CONTAINER $APP_CONTAINER 2>/dev/null || true
docker network rm $NETWORK_NAME 2>/dev/null || true

# 2. Create the Dockerfile (Multi-stage build preserved)
echo "🔨 Creating Dockerfile (Go 1.26)..."
cat <<EOF > Dockerfile
FROM docker.io/library/golang:1.26 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main \$(find ./BatchScheduling/cmd -type f -name "*.go" | head -n 1 | xargs dirname)

FROM docker.io/library/alpine:latest
WORKDIR /app
COPY --from=builder /app/main .
CMD ["./main"]
EOF

# 3. Build Image
echo "📦 Building application image..."
docker build -t $IMAGE_NAME .

# 4. Create Network and Database
echo "🌐 Setting up Docker Network and PostgreSQL..."
docker network create $NETWORK_NAME

docker run -d \
    --name $DB_CONTAINER \
    --network $NETWORK_NAME \
    -p 5432:5432 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=password \
    -e POSTGRES_DB=batch_db \
    docker.io/library/postgres:latest

# Wait for DB
until docker exec $DB_CONTAINER pg_isready -U postgres > /dev/null 2>&1; do
    echo -n "." ; sleep 1
done
echo -e "\n✅ Database Ready."
sleep 5

# 5. Initialize Schema and Seed Data
echo "📝 Initializing schema and seeding test cases..."
docker exec -i $DB_CONTAINER psql -U postgres -d batch_db <<EOF
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS batch_tasks (
    id UUID PRIMARY KEY,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'PENDING',
    attempts INT DEFAULT 0,
    max_attempts INT DEFAULT 5,
    next_run_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_error TEXT,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO batch_tasks (id, payload, status)
SELECT gen_random_uuid(), json_build_object('test_id', s.i, 'msg', 'hello from docker'), 'PENDING'
FROM generate_series(1, $TOTAL_TASKS) s(i);
EOF

# --- VERBOSE: ROWS ADDED ---
ADDED_COUNT=$(docker exec $DB_CONTAINER psql -U postgres -d batch_db -t -c "SELECT count(*) FROM batch_tasks;" | xargs)
echo "➕ VERBOSE: Successfully added $ADDED_COUNT rows to 'batch_tasks'."

# 6. Run the Application
echo "🏃 Running Go App..."
# We pass the DB_URL because in Docker, the DB is at 'batch-db-test', not 'localhost'
docker run -d \
    --name $APP_CONTAINER \
    --network $NETWORK_NAME \
    -e DB_URL="postgres://postgres:password@$DB_CONTAINER:5432/batch_db?sslmode=disable" \
    $IMAGE_NAME

echo "⏳ Waiting 65 seconds for the 1-minute cron trigger..."
sleep 65

# 7. Verify Results & Verbose Reporting
echo "🔍 Analyzing Results..."

COMPLETED=$(docker exec $DB_CONTAINER psql -U postgres -d batch_db -t -c "SELECT count(*) FROM batch_tasks WHERE status = 'COMPLETED';" | xargs)
PENDING=$(docker exec $DB_CONTAINER psql -U postgres -d batch_db -t -c "SELECT count(*) FROM batch_tasks WHERE status = 'PENDING';" | xargs)
MODIFIED=$(docker exec $DB_CONTAINER psql -U postgres -d batch_db -t -c "SELECT count(*) FROM batch_tasks WHERE updated_at > next_run_at;" | xargs)

echo "---------------------------------------"
echo "📊 VERBOSE REPORT:"
echo "---------------------------------------"
echo "✅ ROWS ADDED:    $ADDED_COUNT"
echo "🔄 ROWS MODIFIED: $MODIFIED"
echo "🗑️  ROWS DELETED:  0 (Cleanup not triggered)"
echo "---------------------------------------"
echo "🔢 STATUS SUMMARY:"
echo "✅ COMPLETED: $COMPLETED"
echo "⏳ PENDING:   $PENDING"
echo "---------------------------------------"

# --- VERBOSE: DATA PRINT ---
echo "📑 DATA PREVIEW (First 5 rows):"
docker exec $DB_CONTAINER psql -U postgres -d batch_db -c "SELECT id, status, updated_at FROM batch_tasks LIMIT 5;"

# 8. Final Success Check
if [ "$COMPLETED" -eq "$TOTAL_TASKS" ]; then
    echo ""
    echo "🌟 MISSION ACCOMPLISHED 🌟"
    echo "✅ All $TOTAL_TASKS steps are SUCCESSFUL!"
    echo "---------------------------------------"
else
    echo "❌ TEST FAILED: Expected $TOTAL_TASKS completed, but got $COMPLETED."
    exit 1
fi

# 9. Logs check
echo "📋 Application Logs Preview:"
docker logs --tail 20 $APP_CONTAINER

# Cleanup prompt
echo "---------------------------------------"
read -p "Press enter to cleanup and remove containers/network..."
docker rm -f $DB_CONTAINER $APP_CONTAINER
docker network rm $NETWORK_NAME
echo "🧹 Cleanup complete. All systems clear."
