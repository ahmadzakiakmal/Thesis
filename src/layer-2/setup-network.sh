#!/bin/bash

# Define paths
BASE_DIR="$HOME/go/src/github.com/thesis-prototype/layer-2"
SEQ_DIR="$BASE_DIR/sequencer"
FN1_DIR="$BASE_DIR/fullnode1"
FN2_DIR="$BASE_DIR/fullnode2"
FN3_DIR="$BASE_DIR/fullnode3"

# Define the chain ID
export CHAIN_ID="gm"

# Create directories if they don't exist
mkdir -p $SEQ_DIR/config $SEQ_DIR/data
mkdir -p $FN1_DIR/config $FN1_DIR/data
mkdir -p $FN2_DIR/config $FN2_DIR/data
mkdir -p $FN3_DIR/config $FN3_DIR/data

# Create empty validator state files
echo "{}" > $SEQ_DIR/data/priv_validator_state.json
echo "{}" > $FN1_DIR/data/priv_validator_state.json
echo "{}" > $FN2_DIR/data/priv_validator_state.json
echo "{}" > $FN3_DIR/data/priv_validator_state.json

# Create a minimal rollkit.toml file for each node
for DIR in "$SEQ_DIR" "$FN1_DIR" "$FN2_DIR" "$FN3_DIR"; do
  cat > $DIR/config/rollkit.toml << EOC
[chain]
  config_dir = "$DIR"
EOC
done

# Initialize the sequencer
echo "Initializing sequencer node..."
cd $SEQ_DIR
rollkit init Sequencer --chain-id=$CHAIN_ID --home=$SEQ_DIR

# Verify that priv_validator_key.json was created
if [ -f "$SEQ_DIR/config/priv_validator_key.json" ]; then
  echo "Sequencer priv_validator_key.json created successfully."
else
  echo "ERROR: Sequencer priv_validator_key.json was not created!"
  exit 1
fi

# Get the sequencer's P2P ID for later use
SEQ_NODE_ID=$(rollkit tendermint show-node-id --home=$SEQ_DIR)
echo "Sequencer P2P ID: $SEQ_NODE_ID"
echo $SEQ_NODE_ID > $BASE_DIR/scripts/sequencer_p2p_id.txt

# Initialize the full nodes
for i in 1 2 3; do
  NODE_DIR="$BASE_DIR/fullnode$i"
  echo "Initializing full node $i..."
  cd $NODE_DIR
  rollkit init "FullNode-$i" --chain-id=$CHAIN_ID --home=$NODE_DIR
  
  # Verify that priv_validator_key.json was created
  if [ -f "$NODE_DIR/config/priv_validator_key.json" ]; then
    echo "Full node $i priv_validator_key.json created successfully."
  else
    echo "ERROR: Full node $i priv_validator_key.json was not created!"
    exit 1
  fi
  
  # Copy genesis from sequencer
  cp "$SEQ_DIR/config/genesis.json" "$NODE_DIR/config/genesis.json"
  echo "Genesis file copied to full node $i."
done

echo "All nodes initialized successfully!"
echo "Sequencer P2P ID: $SEQ_NODE_ID"
echo "To start the network:"
echo "1. Start the fake DA server: ./scripts/fake-da-server.sh"
echo "2. Start the sequencer: ./scripts/start-sequencer.sh"
echo "3. Start the full nodes: ./scripts/start-fullnode.sh <node_number> $SEQ_NODE_ID"