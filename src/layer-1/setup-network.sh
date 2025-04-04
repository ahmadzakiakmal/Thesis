#!/bin/bash

# Default values
NODE_COUNT=4
BASE_P2P_PORT=9000
BASE_RPC_PORT=9001
BASE_HTTP_PORT=5000
BASE_DIR="./node-config"
DISABLE_EMPTY_BLOCKS=false
BASE_POSTGRES_PORT=5432

# Parse command line options
while getopts ":n:d:p:r:h:e" opt; do
    case $opt in
    n) NODE_COUNT="$OPTARG" ;;
    d) BASE_DIR="$OPTARG" ;;
    p) BASE_P2P_PORT="$OPTARG" ;;
    r) BASE_RPC_PORT="$OPTARG" ;;
    h) BASE_HTTP_PORT="$OPTARG" ;;
    e) DISABLE_EMPTY_BLOCKS=true ;;
    \?)
        echo "Invalid option -$OPTARG" >&2
        exit 1
        ;;
    esac
done

if [ "$DISABLE_EMPTY_BLOCKS" = true ]; then
    echo "Empty blocks will be disabled"
fi

# Validate node count (minimum 4 for BFT)
if [ $NODE_COUNT -lt 4 ]; then
    echo "Warning: At least 4 nodes are recommended for Byzantine Fault Tolerance."
    echo "The network can only tolerate up to f=(n-1)/3 faulty nodes."
    echo "With $NODE_COUNT nodes, the network cannot tolerate any faults."
fi

echo "Setting up a network with $NODE_COUNT nodes"
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
                PEERS="${NODE_IDS[$j]}@cometbft-node${j}:$p2p_port"
            else
                PEERS="$PEERS,${NODE_IDS[$j]}@cometbft-node${j}:$p2p_port"
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

# Add config for Docker setup
if [ -n "$DOCKER_ENV" ]; then
    echo "Applying Docker-specific configuration..."
    for i in $(seq 0 $((NODE_COUNT - 1))); do
        # Get current peers configuration
        PEERS=$(grep "^persistent_peers =" "$BASE_DIR/node$i/config/config.toml" | sed 's/persistent_peers = "//' | sed 's/"$//')

        # Replace 127.0.0.1 with container names (cometbft-node0, cometbft-node1, etc.)
        # The container name needs to match the node number in the port
        NEW_PEERS=""
        IFS=',' read -ra PEER_ARRAY <<<"$PEERS"
        for peer in "${PEER_ARRAY[@]}"; do
            # Extract node ID and port
            NODE_ID=$(echo "$peer" | cut -d@ -f1)
            PORT=$(echo "$peer" | cut -d: -f2)

            # Calculate node number from port
            NODE_NUM=$(((PORT - BASE_P2P_PORT) / 2))

            # Create new peer address with container name
            if [ -z "$NEW_PEERS" ]; then
                NEW_PEERS="${NODE_ID}@cometbft-node${NODE_NUM}:${PORT}"
            else
                NEW_PEERS="${NEW_PEERS},${NODE_ID}@cometbft-node${NODE_NUM}:${PORT}"
            fi
        done

        # Update the config file with new peers
        sed -i.bak "s|^persistent_peers = \".*\"|persistent_peers = \"$NEW_PEERS\"|" "$BASE_DIR/node$i/config/config.toml"

        echo "Docker-specific peers configured for node$i: $NEW_PEERS"
    done
fi

# Create a docker-compose.yml file
rm -rf
echo "Clearing and creating new docker-compose.yml..."

cat >"./docker-compose.yml" <<EOL
services:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
    p2p_port=$((BASE_P2P_PORT + i * 2))
    rpc_port=$((BASE_RPC_PORT + i * 2))
    http_port=$((BASE_HTTP_PORT + i))
    postgres_port=$((BASE_POSTGRES_PORT + i))

    cat >>"./docker-compose.yml" <<EOL
  cometbft-node$i:
    image: dews-image:latest
    container_name: cometbft-node$i
    ports:
      - "$http_port:$http_port"
      - "$p2p_port:$p2p_port"
      - "$rpc_port:$rpc_port"
    volumes:
      - ./build/bin:/app/bin
      - $BASE_DIR/node$i:/root/.cometbft
    command: >
        sh -c "/app/bin --cmt-home=/root/.cometbft --http-port $http_port --postgres-host=postgres-node$i:5432"
    networks:
      - dews-network
  postgres-node$i:
    image: postgres:14
    container_name: postgres-node$i
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgrespassword
      POSTGRES_DB: dewsdb
    volumes:
      - postgres-data-node$i:/var/lib/postgresql/data
    ports:
      - "$postgres_port:5432"
    networks:
      - dews-network

EOL
done

cat >>"./docker-compose.yml" <<EOL
networks:
  dews-network:
    driver: bridge

volumes:
EOL

for i in $(seq 0 $((NODE_COUNT - 1))); do
  echo "Setting up Postgres container for node $i..."
  cat >> "./docker-compose.yml" << EOL
  postgres-data-node$i:
EOL
done
echo "Done setting up Postgres $i..."

echo "docker-compose.yml created with $NODE_COUNT nodes"
echo "Note: You need to build the dews-image first: docker build -t dews-image:latest ."

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
    echo "Node $i: ./build/bin --cmt-home=$BASE_DIR/node$i --http-port $http_port --postgres-host=postgres-node$i:$postgres_port"
done

echo ""
echo "To check if nodes are connected:"

for i in $(seq 0 $((NODE_COUNT - 1))); do
    http_port=$((BASE_HTTP_PORT + i))
    echo "Node $i: http://localhost:$http_port"
done

# Display persistent peers configuration for each node
# echo ""
# echo "==== Persistent Peers Configuration ===="
# echo ""
# for i in $(seq 0 $((NODE_COUNT-1))); do
#     echo "Node $i persistent peers:"
#     grep "^persistent_peers =" "$BASE_DIR/node$i/config/config.toml"
# done
