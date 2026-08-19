#!/usr/bin/env bash
set -e

echo "Running Control Plane Smoke Test (Binary Blackbox)..."

# Ensure environment is set for the smoke test
export METERGATE_PORT=3000
export METERGATE_CONFIG_PATH="infra/metergate.yml"
export DATABASE_URL=${DATABASE_URL:-"postgresql://metergate:metergate@localhost:5432/metergate?schema=public"}
export REDIS_URL=${REDIS_URL:-"redis://localhost:6379"}

# Check if the server is already running (e.g. via docker-compose)
if curl -s http://localhost:$METERGATE_PORT/health > /dev/null; then
    echo "Server is already running on port $METERGATE_PORT. Testing existing instance..."
    SERVER_PID=""
else
    # Build the Control Plane
    npm run build

    # Start the compiled binary in the background
    node dist/src/controlplane/main.js &
    SERVER_PID=$!
fi

# Function to cleanup the background process on exit
cleanup() {
    if [ -n "$SERVER_PID" ]; then
        echo "Stopping Control Plane (PID: $SERVER_PID)..."
        kill $SERVER_PID || true
    fi
}
trap cleanup EXIT

# Wait for the server to be ready
echo "Waiting for server to listen on port $METERGATE_PORT..."
TIMEOUT=15
while ! curl -s http://localhost:$METERGATE_PORT/health > /dev/null; do
    TIMEOUT=$((TIMEOUT - 1))
    if [ $TIMEOUT -eq 0 ]; then
        echo "Error: Server failed to start within time."
        exit 1
    fi
    sleep 1
done

echo "Server is up!"

# Run Smoke Tests

# 1. Healthcheck
echo "Testing GET /health..."
HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$METERGATE_PORT/health)
if [ "$HEALTH_STATUS" != "200" ]; then
    echo "Failed: GET /health returned $HEALTH_STATUS"
    exit 1
fi
echo "Pass: GET /health returned 200 OK"

# 2. GraphQL Introspection Check (should return 400 since we aren't passing a valid query, but proves endpoint is alive)
echo "Testing POST /graphql..."
CHECK_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d '{}' http://localhost:$METERGATE_PORT/graphql)
if [ "$CHECK_STATUS" != "400" ]; then
    echo "Failed: POST /graphql returned $CHECK_STATUS (expected 400 Bad Request)"
    exit 1
fi
echo "Pass: POST /graphql returned 400 Bad Request"

# 3. OpenTelemetry Metrics Check (should return 200 with Prometheus metrics)
echo "Testing GET /metrics (Port 9464)..."
METRICS_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9464/metrics)
if [ "$METRICS_STATUS" != "200" ]; then
    echo "Failed: GET /metrics returned $METRICS_STATUS"
    exit 1
fi
echo "Pass: GET /metrics returned 200 OK"

# 4. Database Seeding Check
echo "Testing Database Seeding..."
DB_CHECK=$(node -e "
const { PrismaClient } = require('@prisma/client');
const prisma = new PrismaClient();
async function run() {
  try {
    const tenantCount = await prisma.tenant.count();
    const keyCount = await prisma.apiKey.count();
    if (tenantCount === 0 || keyCount === 0) {
      console.log('0');
    } else {
      console.log('1');
    }
  } catch (e) {
    console.log('0');
  } finally {
    await prisma.\$disconnect();
  }
}
run();
")

if [ "$DB_CHECK" != "1" ]; then
    echo "Failed: No tenants or API keys found in database!"
    exit 1
fi
echo "Pass: Found seeded tenant and API Key in database"

echo "Smoke test passed successfully!"
