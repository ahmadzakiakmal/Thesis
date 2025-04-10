#!/bin/bash

# Define paths
SEQ_DIR="$HOME/go/src/github.com/thesis-prototype/layer-2/sequencer"

# Define the chain ID
export CHAIN_ID="gm"

# Start the sequencer node
cd $SEQ_DIR
rollkit start \
  --home="$SEQ_DIR" \
  --rollkit.aggregator \
  --rollkit.da_address "http://127.0.0.1:7980" \
  --rollkit.block_time 1s \
  --rollkit.da_start_height 0 \
  --p2p.laddr "tcp://0.0.0.0:26656" \
  --rpc.laddr "tcp://127.0.0.1:26657"
