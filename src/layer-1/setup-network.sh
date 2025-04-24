#!/bin/bash

# Default values
NODE_COUNT=4
BASE_P2P_PORT=9001
BASE_RPC_PORT=9000
BASE_HTTP_PORT=5000
BASE_DIR="./node-config"
DISABLE_EMPTY_BLOCKS=false
BASE_POSTGRES_PORT=5440
MODE="prod"  # Default to production mode

# Parse command line options
while getopts ":n:d:p:r:h:e:m:" opt; do
    case $opt in
    n) NODE_COUNT="$OPTARG" ;;
    d) BASE_DIR="$OPTARG" ;;
    p) BASE_P2P_PORT="$OPTARG" ;;
    r) BASE_RPC_PORT="$OPTARG" ;;
    h) BASE_HTTP_PORT="$OPTARG" ;;
    e) DISABLE_EMPTY_BLOCKS=true ;;
    m) MODE="$OPTARG" ;;
    \?)
        echo "Invalid option -$OPTARG" >&2
        exit 1
        ;;
    esac
done

# Validate mode
if [ "$MODE" != "prod" ] && [ "$MODE" != "dev" ]; then
    echo "Invalid mode: $MODE. Mode must be 'prod' or 'dev'"
    exit 1
fi

if [ "$DISABLE_EMPTY_BLOCKS" = true ]; then
    echo "Empty blocks will be disabled"
fi

# Validate node count (minimum 4 for BFT)
if [ $NODE_COUNT -lt 4 ]; then
    echo "Warning: At least 4 nodes are recommended for Byzantine Fault Tolerance."
    echo "The network can only tolerate up to f=(n-1)/3 faulty nodes."
    echo "With $NODE_COUNT nodes, the network cannot tolerate any faults."
fi

echo "Setting up a network with $NODE_COUNT nodes in $MODE mode"
echo "Base directory: $BASE_DIR"
echo "Base P2P port: $BASE_P2P_PORT"
echo "Base RPC port: $BASE_RPC_PORT"
echo "Base HTTP port: $BASE_HTTP_PORT"

# Create base directory if it doesn't exist
mkdir -p "$BASE_DIR"

# Clear existing configuration
rm -rf "$BASE_DIR"/node*

# Create directory for each node
for i in $(seq 0 $((NODE_COUNT - 1))); do
    mkdir -p "$BASE_DIR/node$i"
    echo "Created directory for node$i"
done

# Initialize nodes
echo "Initializing nodes..."

for i in $(seq 0 $((NODE_COUNT - 1))); do
    cometbft init --home="$BASE_DIR/node$i"
    # Set moniker for each node
    sed -i.bak "s/^moniker = \".*\"/moniker = \"node$i\"/" "$BASE_DIR/node$i/config/config.toml"
    echo "Node $i initialized with moniker 'node$i'"
done

# Configure ports for each node
for i in $(seq 0 $((NODE_COUNT - 1))); do
    p2p_port=$((BASE_P2P_PORT + i * 2))
    rpc_port=$((BASE_RPC_PORT + i * 2))

    sed -i.bak "s/^laddr = \"tcp:\/\/0.0.0.0:26656\"/laddr = \"tcp:\/\/0.0.0.0:$p2p_port\"/" "$BASE_DIR/node$i/config/config.toml"
    sed -i.bak "s/^laddr = \"tcp:\/\/127.0.0.1:26657\"/laddr = \"tcp:\/\/0.0.0.0:$rpc_port\"/" "$BASE_DIR/node$i/config/config.toml"
    echo "Node $i configured to use P2P port $p2p_port and RPC port $rpc_port"
done

if [ "$DISABLE_EMPTY_BLOCKS" = true ]; then
    for i in $(seq 0 $((NODE_COUNT - 1))); do
        # Disable creating empty blocks
        sed -i.bak 's/^create_empty_blocks = true/create_empty_blocks = false/' "$BASE_DIR/node$i/config/config.toml"
        echo "Node $i configured to not create empty blocks"
    done
fi

# Get validator info from the first node
echo "Extracting validator info from the first node"
FIRST_NODE_VALIDATOR=$(cat "$BASE_DIR/node0/config/genesis.json" | jq '.validators[0]')

# Create updated genesis with validators from all nodes
echo "Creating updated genesis with validators from all nodes"
cp "$BASE_DIR/node0/config/genesis.json" "$BASE_DIR/updated_genesis.json"

# Add validators from all nodes to the genesis
for i in $(seq 1 $((NODE_COUNT - 1))); do
    NODE_PUBKEY=$(cat "$BASE_DIR/node$i/config/priv_validator_key.json" | jq -r '.pub_key.value')
    cat "$BASE_DIR/updated_genesis.json" | jq --arg pubkey "$NODE_PUBKEY" --arg name "node$i" \
        '.validators += [{"address":"","pub_key":{"type":"tendermint/PubKeyEd25519","value":$pubkey},"power":"10","name":$name}]' >"$BASE_DIR/temp_genesis.json"
    mv "$BASE_DIR/temp_genesis.json" "$BASE_DIR/updated_genesis.json"
done

# Copy updated genesis to all nodes
echo "Sharing updated genesis file to all nodes"
for i in $(seq 0 $((NODE_COUNT - 1))); do
    cp "$BASE_DIR/updated_genesis.json" "$BASE_DIR/node$i/config/genesis.json"
done
echo "Updated genesis file with $NODE_COUNT validators successfully shared to all nodes"

# Get node IDs
echo "Getting node IDs"
declare -a NODE_IDS
for i in $(seq 0 $((NODE_COUNT - 1))); do
    NODE_IDS[$i]=$(cometbft show-node-id --home="$BASE_DIR/node$i")
    echo "Node$i ID: ${NODE_IDS[$i]}"
done

# Configure persistent peers for each node - FULL MESH CONFIGURATION
echo "Configuring full mesh peer connections..."

for i in $(seq 0 $((NODE_COUNT - 1))); do
    PEERS=""
    for j in $(seq 0 $((NODE_COUNT - 1))); do
        if [ $i -ne $j ]; then
            p2p_port=$((BASE_P2P_PORT + j * 2))
            if [ -z "$PEERS" ]; then
                PEERS="${NODE_IDS[$j]}@layer-1-node${j}:$p2p_port"
            else
                PEERS="$PEERS,${NODE_IDS[$j]}@layer-1-node${j}:$p2p_port"
            fi
        fi
    done

    sed -i.bak "s/^persistent_peers = \"\"/persistent_peers = \"$PEERS\"/" "$BASE_DIR/node$i/config/config.toml"
    echo "Node $i configured to connect to peers: $PEERS"
done

# Configure each node for local development
for i in $(seq 0 $((NODE_COUNT - 1))); do
    # Allow non-safe connections (for development only)
    sed -i.bak 's/^addr_book_strict = true/addr_book_strict = false/' "$BASE_DIR/node$i/config/config.toml"

    # Allow CORS for web server access
    sed -i.bak 's/^cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/' "$BASE_DIR/node$i/config/config.toml"

    echo "Local development settings configured for node$i"
done

# Create docker-compose files for both development and production
echo "Clearing and creating new docker-compose files..."

# Generate docker-compose.dev.yml for development
cat > "./docker-compose.dev.yml" << EOL
services:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
    p2p_port=$((BASE_P2P_PORT + i * 2))
    rpc_port=$((BASE_RPC_PORT + i * 2))
    http_port=$((BASE_HTTP_PORT + i))
    postgres_port=$((BASE_POSTGRES_PORT + i))

    cat >> "./docker-compose.dev.yml" << EOL
  layer-1-node$i:
    image: layer1-node-dev:latest
    container_name: layer-1-node$i
    ports:
      - "$http_port:$http_port"
      - "$p2p_port:$p2p_port"
      - "$rpc_port:$rpc_port"
    volumes:
      - ./build/bin:/app/bin
      - $BASE_DIR/node$i:/root/.cometbft
    command: >
        sh -c "/app/bin --cmt-home=/root/.cometbft --http-port $http_port --postgres-host=layer-1-postgres$i:5432"
    networks:
      - bft-ws-network
  layer-1-postgres$i:
    image: postgres:14
    container_name: layer-1-postgres$i
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgrespassword
      POSTGRES_DB: dewsdb
    volumes:
      - postgres-data-node$i:/var/lib/postgresql/data
    ports:
      - "$postgres_port:5432"
    networks:
      - bft-ws-network

EOL
done

cat >> "./docker-compose.dev.yml" << EOL
networks:
  bft-ws-network:
    name: bft-ws-network
    driver: bridge
    external: false

volumes:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
  cat >> "./docker-compose.dev.yml" << EOL
  postgres-data-node$i:
EOL
done

# Generate docker-compose.yml for production
cat > "./docker-compose.yml" << EOL
services:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
    p2p_port=$((BASE_P2P_PORT + i * 2))
    rpc_port=$((BASE_RPC_PORT + i * 2))
    http_port=$((BASE_HTTP_PORT + i))
    postgres_port=$((BASE_POSTGRES_PORT + i))

    cat >> "./docker-compose.yml" << EOL
  layer-1-node$i:
    image: layer1-node:latest
    container_name: layer-1-node$i
    ports:
      - "$http_port:$http_port"
      - "$p2p_port:$p2p_port"
      - "$rpc_port:$rpc_port"
    volumes:
      - $BASE_DIR/node$i:/root/.cometbft
    command: >
        sh -c "/app/bin --cmt-home=/root/.cometbft --http-port $http_port --postgres-host=layer-1-postgres$i:5432"
    networks:
      - bft-ws-network
  layer-1-postgres$i:
    image: postgres:14
    container_name: layer-1-postgres$i
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgrespassword
      POSTGRES_DB: dewsdb
    volumes:
      - postgres-data-node$i:/var/lib/postgresql/data
    ports:
      - "$postgres_port:5432"
    networks:
      - bft-ws-network

EOL
done

cat >> "./docker-compose.yml" << EOL
networks:
  bft-ws-network:
    name: bft-ws-network
    driver: bridge
    external: false

volumes:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
  cat >> "./docker-compose.yml" << EOL
  postgres-data-node$i:
EOL
done

echo "docker-compose files created with $NODE_COUNT nodes"
echo "Note: You need to build the appropriate Docker image first:"
echo "For dev mode: docker build -f Dockerfile.dev -t layer1-node-dev:latest ."
echo "For prod mode: docker build -f Dockerfile -t layer1-node:latest ."

# Fix permissions for Docker access
echo "Setting appropriate permissions for Docker..."
sudo chown -R $(id -u):$(id -g) node-config/
sudo chmod -R 777 node-config/
# Ensure any badger directories are writable
for i in $(seq 0 $((NODE_COUNT - 1))); do
    if [ -d "$BASE_DIR/node$i/badger" ]; then
        sudo chmod -R a+rw "$BASE_DIR/node$i/badger"
    fi
done
echo "Permissions set correctly"

# Clear any existing data directories to avoid genesis hash mismatch
echo "Clearing existing data directories..."
for i in $(seq 0 $((NODE_COUNT - 1))); do
    # Create the data directory if it doesn't exist
    mkdir -p "$BASE_DIR/node$i/data"

    # Clear contents but ensure priv_validator_state.json exists
    rm -rf "$BASE_DIR/node$i/data/"*

    # Create an empty priv_validator_state.json file
    echo '{
        "height": "0",
        "round": 0,
        "step": 0
    }' >"$BASE_DIR/node$i/data/priv_validator_state.json"

    echo "Node $i data directory reset"
done

# Display startup instructions
echo ""
echo "==== Network Setup Complete ===="
echo ""

for i in $(seq 0 $((NODE_COUNT - 1))); do
    http_port=$((BASE_HTTP_PORT + i))
    postgres_port=$((BASE_POSTGRES_PORT + 1))
    echo "Node $i: ./build/bin --cmt-home=$BASE_DIR/node$i --http-port $http_port --postgres-host=layer-1-postgres$i:$postgres_port"
done

echo ""
echo "To check if nodes are connected:"

for i in $(seq 0 $((NODE_COUNT - 1))); do
    http_port=$((BASE_HTTP_PORT + i))
    echo "Node $i: http://localhost:$http_port"
done

# Display mode-specific instructions
echo ""
if [ "$MODE" = "dev" ]; then
    echo "Development mode is active. Generated docker-compose.dev.yml which mounts local binary."
    echo "To run using volume sharing: make dev-fast"
    echo "To run using docker image: make dev"
else
    echo "Production mode is active. Generated docker-compose.yml which uses pre-built binary in the image."
    echo "To run: make prod"
fi