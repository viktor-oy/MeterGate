#!/bin/bash
set -e

OVERRIDE=0
for arg in "$@"; do
    if [ "$arg" == "--override-already-running" ]; then
        OVERRIDE=1
    fi
done

# Define default ports
CP_PORT=3000
DP_PORT=8080


if [ "$OVERRIDE" -eq 1 ]; then
    echo "Checking and terminating processes on required ports..."
    # Nuke any orphaned parent watcher processes (the Hydra problem)
    pkill -f 'nest start --watch' || true
    pkill -f 'controlplane/main' || true
    
    for PORT in $CP_PORT $DP_PORT; do
        PIDS=$(lsof -t -i:$PORT || true)
        if [ ! -z "$PIDS" ]; then
            echo "Killing processes on port $PORT: $(echo $PIDS | tr '\n' ' ')"
            echo "$PIDS" | xargs kill -9 2>/dev/null || true
        fi
    done
else
    # Fail-fast check if ports are already in use
    for PORT in $CP_PORT $DP_PORT; do
        PID=$(lsof -t -i:$PORT || true)
        if [ ! -z "$PID" ]; then
            echo "================================================="
            echo "ERROR: Port $PORT is already in use by process $PID!"
            echo "You can run 'make dev OVERRIDE=1' to forcefully kill it."
            echo "================================================="
            exit 1
        fi
    done
fi

echo "Starting infrastructure dependencies..."
make infra-deps-up

echo "Starting Control Plane..."
export DATABASE_URL="postgresql://metergate:metergate@localhost:5432/metergate?schema=public"
mise x -- npm run start:dev &
CP_PID=$!

echo "Starting Data Plane..."
# We pass environment variables so data plane runs on its dedicated port
cd src/dataplane && METERGATE_PORT=$DP_PORT METERGATE_CONFIG_PATH="../../infra/metergate.yml" REDIS_HOST="localhost:6379" mise x -- go run cmd/dataplane/main.go &
DP_PID=$!

echo ""
echo "================================================="
echo "Development environment is natively running!"
echo "Control Plane PID: $CP_PID (Port $CP_PORT)"
echo "Data Plane PID: $DP_PID (Port: $DP_PORT)"
echo "Press Ctrl+C to shutdown."
echo "================================================="
echo ""

# Trap shutdown signals
trap 'echo -e "\nStopping servers..."; kill $CP_PID $DP_PID 2>/dev/null || true; exit 0' SIGINT SIGTERM

# Monitor processes (compatible with macOS bash 3.2 which lacks wait -n)
while true; do
    if ! kill -0 $CP_PID 2>/dev/null; then
        echo -e "\nControl Plane exited prematurely. Shutting down..."
        kill $DP_PID 2>/dev/null || true
        exit 1
    fi
    if ! kill -0 $DP_PID 2>/dev/null; then
        echo -e "\nData Plane exited prematurely. Shutting down..."
        kill $CP_PID 2>/dev/null || true
        exit 1
    fi
    sleep 1
done
