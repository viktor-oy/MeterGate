#!/usr/bin/env bash
set -e

echo "Running Control Plane Smoke Test (Binary Blackbox)..."

export CP_REPLICAS=${CP_REPLICAS:-1}
export DP_REPLICAS=${DP_REPLICAS:-1}

export METERGATE_CONFIG_PATH="infra/metergate.yml"
export DATABASE_URL=${DATABASE_URL:-"postgresql://metergate:metergate@localhost:5432/metergate?schema=public"}
export REDIS_URL=${REDIS_URL:-"redis://localhost:6379"}
export METERGATE_KEY_HASH_SECRET="local-demo-secret-change-me"

CP_PIDS=""
DP_PIDS=""

	npm run build
	for (( i=0; i<CP_REPLICAS; i++ )); do
		PORT=$((3100 + i))
		echo "Starting Control Plane on port $PORT..."
		METERGATE_PORT=$PORT node dist/src/controlplane/main.js &
		CP_PIDS="$CP_PIDS $!"
	done

cleanup() {
    for pid in $CP_PIDS; do
        echo "Stopping Control Plane (PID: $pid)..."
        kill $pid || true
    done
    for pid in $DP_PIDS; do
        echo "Stopping Data Plane (PID: $pid)..."
        kill $pid || true
    done
}
trap cleanup EXIT

for (( i=0; i<CP_REPLICAS; i++ )); do
    PORT=$((3100 + i))
    echo "Waiting for Control Plane to listen on port $PORT..."
    TIMEOUT=30
    while ! curl -s http://localhost:$PORT/health > /dev/null; do
        TIMEOUT=$((TIMEOUT - 1))
        if [ $TIMEOUT -eq 0 ]; then
            echo "Error: Server on $PORT failed to start within time."
            exit 1
        fi
        sleep 1
    done
    echo "Control Plane on $PORT is up!"
done

echo "Building and starting Data Plane..."
cd src/dataplane
export REDIS_HOST="localhost:6379"
export DISABLE_ZERO_ALLOC="false"
mise x -- go build -o ../../bin/test-dataplane cmd/dataplane/main.go
cd ../..

for (( i=0; i<DP_REPLICAS; i++ )); do
    PROXY_PORT=$((8180 + i))
    METRICS_PORT=$((6160 + i))
    echo "Starting Data Plane on proxy port $PROXY_PORT, metrics port $METRICS_PORT..."
    METERGATE_PORT=$PROXY_PORT METERGATE_METRICS_PORT=$METRICS_PORT ./bin/test-dataplane &
    DP_PIDS="$DP_PIDS $!"
done

for (( i=0; i<DP_REPLICAS; i++ )); do
    METRICS_PORT=$((6160 + i))
    echo "Waiting for Data Plane to listen on port $METRICS_PORT..."
    TIMEOUT=30
    while ! curl -s http://localhost:$METRICS_PORT/health > /dev/null; do
        TIMEOUT=$((TIMEOUT - 1))
        if [ $TIMEOUT -eq 0 ]; then
            echo "Error: Data Plane on $METRICS_PORT failed to start within time."
            exit 1
        fi
        sleep 1
    done
    echo "Data Plane on $METRICS_PORT is up!"
done

for (( i=0; i<CP_REPLICAS; i++ )); do
    PORT=$((3100 + i))
    echo "Testing GET /health (Control Plane on $PORT)..."
    HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$PORT/health)
    if [ "$HEALTH_STATUS" != "200" ]; then
        echo "Failed: GET /health returned $HEALTH_STATUS"
        exit 1
    fi
    echo "Pass: GET /health (Control Plane) returned 200 OK"
    
    echo "Testing POST /graphql (Control Plane on $PORT)..."
    CHECK_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d '{}' http://localhost:$PORT/graphql)
    if [ "$CHECK_STATUS" != "400" ]; then
        echo "Failed: POST /graphql returned $CHECK_STATUS (expected 400 Bad Request)"
        exit 1
    fi
    echo "Pass: POST /graphql returned 400 Bad Request"
done

for (( i=0; i<DP_REPLICAS; i++ )); do
    METRICS_PORT=$((6160 + i))
    echo "Testing GET /health (Data Plane on $METRICS_PORT)..."
    DP_HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$METRICS_PORT/health)
    if [ "$DP_HEALTH_STATUS" != "200" ]; then
        echo "Failed: GET /health (Data Plane) returned $DP_HEALTH_STATUS"
        exit 1
    fi
    echo "Pass: GET /health (Data Plane) returned 200 OK"

    echo "Testing GET /metrics (Data Plane on $METRICS_PORT)..."
    DP_METRICS_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$METRICS_PORT/metrics)
    if [ "$DP_METRICS_STATUS" != "200" ]; then
        echo "Failed: GET /metrics (Data Plane) returned $DP_METRICS_STATUS"
        exit 1
    fi
    echo "Pass: GET /metrics (Data Plane) returned 200 OK"
done

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
