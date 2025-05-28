\#!/bin/bash

# Configuration variables
export L1_NODES=4
export L2_NODES=1
export DEV=true
export REBUILD=false

# Base directories
L1_DIR="./layer-1"
L2_DIR="./layer-2"

# Print header
echo "====================================="
echo "  Running System..."
echo "====================================="
echo "Configuration:"
echo "- Layer 1 Nodes: $L1_NODES"
echo "- Layer 2 Nodes: $L2_NODES"
echo "- Development Mode: $DEV"
echo "- Rebuild Images: $REBUILD"
echo "====================================="

# Clean docker environment
echo "Cleaning Docker environment..."
(cd "$L2_DIR" && make docker-clean)
(cd "$L1_DIR" && make docker-clean)

# Create network if it doesn't exist
# echo "Creating Docker network..."
# docker network create bft-ws-network 2>/dev/null || true

# Run layer 1 first
echo "Starting Layer 1..."
(cd "$L1_DIR" && make dev-fast NODES=$L1_NODES DEV=$DEV REBUILD=$REBUILD)

# Wait for Layer 1 to be ready (adjust time as needed)
echo "Waiting for Layer 1 to initialize..."
sleep 2

# Run layer 2
echo "Starting Layer 2..."
(cd "$L2_DIR" && make dev-fast NODES=$L2_NODES DEV=$DEV REBUILD=$REBUILD)

echo "====================================="
echo "System is running"
echo "Layer 1 API is available at: http://localhost:5000"
echo "Layer 2 API is available at: http://localhost:4000"
echo "====================================="

# Optional: Watch logs from both layers
if [ "$1" == "--logs" ]; then
  echo "Showing logs from both layers (Ctrl+C to exit)..."
  docker logs -f layer-1-node0 &
  PID1=$!
  docker logs -f cometbft-simulator &
  PID2=$!
  
  # Handle Ctrl+C gracefully
  trap "kill $PID1 $PID2" SIGINT
  wait
fi