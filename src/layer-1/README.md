# DeWS Replica

An implementation replica for [DeWS: Decentralized and Byzantine Fault-tolerant Web Services](https://ieeexplore.ieee.org/document/10174949/). This implementation uses Go and [CometBFT](https://github.com/cometbft/cometbft), the successor of Tendermint, to provide Byzantine Fault Tolerance.

## What is DeWS?

DeWS introduces a new web service architecture with the following properties:

1. **Decentralized**: Web services run across multiple domains, each operated by different stakeholders.
2. **Byzantine Fault-tolerant**: Can tolerate up to f=(n-1)/3 faulty or malicious nodes.
3. **Transparent**: All requests and responses are logged to an immutable blockchain.
4. **Auditable**: Clients can verify the integrity of responses via blockchain references.

The traditional "request-compute-response" model is replaced with a "request-compute-consensus-log-response" architecture.

## Prerequisites

- Go 1.23+
- CometBFT (installed via `go install github.com/cometbft/cometbft/cmd/cometbft@latest`)
- jq (for processing JSON in setup scripts)

For Docker setup:
- Docker
- Docker Compose

## Setup and Running

### For Dev

Run single validator system locally

```
./setup-network.sh -n 1

# Optional: Disable empty block creation so the log doesnt get too crowded
./setup-network.sh -n 1 -e

# Run manually
go build -o ./build/bin
./build/bin --cmt-home=./node-config/node0

# Or use Make
make run-dev
```

### Flexible Setup

The system supports flexible configuration with a variable number of nodes:

```bash
# Get dependencies and build the go binary
go mod tidy
go build -o ./build/bin

# Build the Docker image
docker build -t dews-image:latest .

# Setup with default 4 nodes
./setup-network.sh

# Setup with custom number of nodes
./setup-network.sh -n 7

# Setup with custom ports
./setup-network.sh -n 5 -p 8000 -r 8001 -h 4000

# Use docker compose
docker-compose up
```

A network can tolerate up to f=(n-1)/3 Byzantine faults, where n is the number of nodes. For example:
- 4 nodes: tolerates 1 fault
- 7 nodes: tolerates 2 faults
- 10 nodes: tolerates 3 faults

This will create and start all nodes in separate containers with appropriate networking.

### Convenient Make Commands

A Makefile is provided for common operations:

```bash
# Build the source code binary
make build-bin

# Run 1 node locally for development
make run-dev

# Run with Docker, 4 nodes
make run-4

# Run with Docker, 10 nodes
make run-10

# Run with Docker, 15 nodes
make run-15

# Clean docker environments
make docker-clean
```

## API Endpoints

The system provides the following REST APIs:

### Debug Endpoint
- `GET /debug` - View node informations to help debug

### Customer Management
- `POST /api/customers` - Create a new customer
- `GET /api/customers` - Get all customers

### Transaction Verification
- `GET /status/{txID}` - Check transaction status

## Byzantine Fault Tolerance Testing

To test Byzantine fault tolerance:

1. Start all nodes
2. Make a request to any node
3. Verify that the transaction reaches consensus and appears in all nodes
4. Modify one node's response logic to produce incorrect results
5. Observe that malicious responses are rejected by the network

## Architecture

The system follows the DeWS architecture described in the paper:

1. **Request**: Client sends an HTTP request to a web server
2. **Compute**: The server processes the request locally
3. **Consensus**: The request and response are broadcast to all nodes for validation
4. **Log**: The transaction is recorded in the blockchain
5. **Response**: Client receives the response with blockchain reference

## Troubleshooting

### Common Issues

1. **Nodes not connecting:**
   - Check that all P2P ports are accessible
   - Verify persistent_peers configuration
   - Use Docker setup for more reliable networking

2. **Consensus failures:**
   - Check logs for error messages
   - Ensure at least 2/3 of nodes are online
   - Verify all nodes have the same genesis file

### Viewing Logs

For Docker setup:
```
docker logs cometbft-node0
```

## License

This is an educational implementation based on the DeWS paper. Use in production environments at your own risk.

## Acknowledgements

- [Original DeWS Paper](https://ieeexplore.ieee.org/document/10174949/)
- [CometBFT](https://github.com/cometbft/cometbft)