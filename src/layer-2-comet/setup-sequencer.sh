#!/bin/bash

# Default values
BASE_DIR="./node-config"
HTTP_PORT=5100
RPC_PORT=26657
P2P_PORT=26656
POSTGRES_PORT=5432
L1_ENDPOINT="http://localhost:5000"
DISABLE_EMPTY_BLOCKS=false
CHAIN_ID="dews-layer2"
BUILD_BINARY=true
LAUNCH_NODE=false

# Parse command line options
while getopts ":d:h:r:p:l:ebs" opt; do
    case $opt in
    d) BASE_DIR="$OPTARG" ;;
    h) HTTP_PORT="$OPTARG" ;;
    r) RPC_PORT="$OPTARG" ;;
    p) P2P_PORT="$OPTARG" ;;
    l) L1_ENDPOINT="$OPTARG" ;;
    e) DISABLE_EMPTY_BLOCKS=true ;;
    b) BUILD_BINARY=true ;;
    s) LAUNCH_NODE=true ;;
    \?)
        echo "Invalid option -$OPTARG" >&2
        exit 1
        ;;
    esac
done

echo "Setting up Layer 2 sequencer node"
echo "Base directory: $BASE_DIR"
echo "HTTP port: $HTTP_PORT"
echo "RPC port: $RPC_PORT"
echo "P2P port: $P2P_PORT"
echo "L1 endpoint: $L1_ENDPOINT"
if [ "$DISABLE_EMPTY_BLOCKS" = true ]; then
    echo "Empty blocks will be disabled"
fi

# Check if cometbft is installed
if ! command -v cometbft &> /dev/null; then
    echo "cometbft command could not be found"
    echo "Please install CometBFT first: go install github.com/cometbft/cometbft/cmd/cometbft@latest"
    exit 1
fi

# Create base directory if it doesn't exist
mkdir -p "$BASE_DIR"

# Clear existing configuration
rm -rf "$BASE_DIR"/*

# Create directory for the node
mkdir -p "$BASE_DIR/config"
mkdir -p "$BASE_DIR/data"

# Initialize the node
echo "Initializing sequencer node..."
cometbft init --home="$BASE_DIR"

# Set moniker
sed -i.bak "s/^moniker = \".*\"/moniker = \"l2-sequencer\"/" "$BASE_DIR/config/config.toml"
echo "Node initialized with moniker 'l2-sequencer'"

# Configure ports
sed -i.bak "s/^laddr = \"tcp:\/\/0.0.0.0:26656\"/laddr = \"tcp:\/\/0.0.0.0:$P2P_PORT\"/" "$BASE_DIR/config/config.toml"
sed -i.bak "s/^laddr = \"tcp:\/\/127.0.0.1:26657\"/laddr = \"tcp:\/\/0.0.0.0:$RPC_PORT\"/" "$BASE_DIR/config/config.toml"
echo "Node configured to use P2P port $P2P_PORT and RPC port $RPC_PORT"

# Configure for faster block times (Layer 2 should be faster)
sed -i.bak 's/^timeout_commit = "5s"/timeout_commit = "1s"/' "$BASE_DIR/config/config.toml"
sed -i.bak 's/^timeout_propose = "3s"/timeout_propose = "1s"/' "$BASE_DIR/config/config.toml"

if [ "$DISABLE_EMPTY_BLOCKS" = true ]; then
    # Disable creating empty blocks
    sed -i.bak 's/^create_empty_blocks = true/create_empty_blocks = false/' "$BASE_DIR/config/config.toml"
    echo "Node configured to not create empty blocks"
else
    # For Layer 2, we want faster empty blocks
    sed -i.bak 's/^create_empty_blocks_interval = "0s"/create_empty_blocks_interval = "1s"/' "$BASE_DIR/config/config.toml"
    echo "Node configured for 1s empty block interval"
fi

# Modify genesis to have a unique chain ID for Layer 2
cat "$BASE_DIR/config/genesis.json" | jq --arg chainid "$CHAIN_ID" '.chain_id = $chainid' > "$BASE_DIR/config/temp_genesis.json"
mv "$BASE_DIR/config/temp_genesis.json" "$BASE_DIR/config/genesis.json"
echo "Genesis updated with chain ID: $CHAIN_ID"

# Allow CORS for web server access
sed -i.bak 's/^cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/' "$BASE_DIR/config/config.toml"

# Create empty validator state
echo '{
    "height": "0",
    "round": 0,
    "step": 0
}' > "$BASE_DIR/data/priv_validator_state.json"

# Create build directory
mkdir -p "./build"

# Create a docker-compose.yml file that only uses volumes (no image building)
echo "Creating docker-compose.yml..."

cat > "./docker-compose.yml" << EOL
services:
  postgres-l2:
    image: postgres:14
    container_name: postgres-l2
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgrespassword
      POSTGRES_DB: dewsdb
    volumes:
      - postgres-data-l2:/var/lib/postgresql/data
    ports:
      - "$POSTGRES_PORT:5432"
    networks:
      - l2-network

networks:
  l2-network:
    driver: bridge
volumes:
  postgres-data-l2:
EOL

echo "docker-compose.yml created for PostgreSQL only"

# Create build script
echo "Creating build script..."

cat > "./build-l2.sh" << EOL
#!/bin/bash

# Create build directory if it doesn't exist
mkdir -p ./build

# Build the binary
echo "Building Layer 2 node binary..."
go build -o ./build/bin .

if [ \$? -eq 0 ]; then
  echo "Build successful!"
  echo "Binary created at ./build/bin"
else
  echo "Build failed!"
  exit 1
fi
EOL

chmod +x ./build-l2.sh
echo "Build script created: ./build-l2.sh"

# Create run script for local execution
echo "Creating run script for local execution..."

cat > "./run-l2.sh" << EOL
#!/bin/bash

# Start PostgreSQL if not running
if ! docker ps | grep -q postgres-l2; then
  echo "Starting PostgreSQL container..."
  docker-compose up -d
  
  # Wait for PostgreSQL to be ready
  echo "Waiting for PostgreSQL to be ready..."
  sleep 5
fi

# Run the node locally
./build/bin --cmt-home=$BASE_DIR \
  --http-port=$HTTP_PORT \
  --postgres-host=localhost:$POSTGRES_PORT \
  --l1-endpoint=$L1_ENDPOINT \
  --sequencer=true \
  --batch-size=10 \
  --batch-time=2 \
  --commit-interval=30 \
  --enable-commits=true
EOL

chmod +x ./run-l2.sh
echo "Run script created: ./run-l2.sh"

# Create a Makefile for convenience
echo "Creating Makefile..."

cat > "./Makefile" << EOL
.PHONY: all build run clean

build:
	@echo "Building binary..."
	@./build-l2.sh

run: build
	@echo "Running Layer 2 node..."
	@./run-l2.sh

clean:
	@echo "Cleaning up..."
	@docker-compose down -v
	@rm -rf ./build

db-start:
	@echo "Starting PostgreSQL database..."
	@docker-compose up -d

db-stop:
	@echo "Stopping PostgreSQL database..."
	@docker-compose down
EOL

echo "Makefile created"

# Build the binary if specified
if [ "$BUILD_BINARY" = true ]; then
    echo "Building the Layer 2 binary..."
    ./build-l2.sh
    
    if [ $? -ne 0 ]; then
        echo "Failed to build binary. Check for errors."
        exit 1
    fi
fi

# Launch the node if specified
if [ "$LAUNCH_NODE" = true ]; then
    echo "Launching the Layer 2 node..."
    ./run-l2.sh &
    echo "Node started in background"
fi

# Fix permissions for Docker access
echo "Setting appropriate permissions for Docker..."
chmod -R 777 "$BASE_DIR"
echo "Permissions set correctly"

echo ""
echo "Layer 2 sequencer node setup complete!"
echo ""
echo "You can now:"
echo "1. Build the binary: make build"
echo "2. Run the node: make run"
echo "3. Clean up: make clean"
echo "4. Start PostgreSQL only: make db-start"
echo "5. Stop PostgreSQL: make db-stop"
echo ""
echo "The Layer 2 sequencer will:"
echo "- Listen for API requests on port $HTTP_PORT"
echo "- Use PostgreSQL on port $POSTGRES_PORT"
echo "- Connect to Layer 1 at $L1_ENDPOINT"
echo "- Process transactions in batches and commit to Layer 1"