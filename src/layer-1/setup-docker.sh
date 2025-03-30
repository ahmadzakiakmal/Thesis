#!/bin/bash

# Default values
NODE_COUNT=4
BASE_P2P_PORT=9000
BASE_RPC_PORT=9001
BASE_HTTP_PORT=5000

# Parse command line options
while getopts ":n:p:r:h:" opt; do
  case $opt in
    n) NODE_COUNT="$OPTARG";;
    p) BASE_P2P_PORT="$OPTARG";;
    r) BASE_RPC_PORT="$OPTARG";;
    h) BASE_HTTP_PORT="$OPTARG";;
    \?) echo "Invalid option -$OPTARG" >&2; exit 1;;
  esac
done

# Validate node count (minimum 4 for BFT)
if [ $NODE_COUNT -lt 4 ]; then
    echo "Warning: At least 4 nodes are recommended for Byzantine Fault Tolerance."
    echo "The network can only tolerate up to f=(n-1)/3 faulty nodes."
    echo "With $NODE_COUNT nodes, the network cannot tolerate any faults."
    
    # Ask for confirmation
    read -p "Do you want to continue with $NODE_COUNT nodes? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

echo "Setting up a Docker network with $NODE_COUNT nodes"
echo "Base P2P port: $BASE_P2P_PORT"
echo "Base RPC port: $BASE_RPC_PORT"
echo "Base HTTP port: $BASE_HTTP_PORT"

# Run the network setup script with Docker settings
DOCKER_ENV=1 CREATE_DOCKER=1 ./setup-network-dynamic.sh -n $NODE_COUNT -p $BASE_P2P_PORT -r $BASE_RPC_PORT -h $BASE_HTTP_PORT

# Create Dockerfile if it doesn't exist
if [ ! -f "./Dockerfile" ]; then
    cat > "./Dockerfile" << EOL
FROM golang:1.21-alpine as builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git build-base

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN go build -o DeWS-Replica

# Create the final image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/DeWS-Replica /app/

# Create directory for templates
COPY templates /app/templates

# Install dependencies for runtime
RUN apk add --no-cache ca-certificates jq bash curl

EXPOSE 5000-6000 9000-9100

CMD ["/app/DeWS-Replica"]
EOL
    echo "Created Dockerfile"
fi

# Create templates directory if it doesn't exist
mkdir -p templates

# Create a simple makefile for convenience
cat > "Makefile" << EOL
.PHONY: build run-local run-docker clean

build:
	go build -o ./build/DeWS-Replica

run-local:
	./setup-network-dynamic.sh -n ${NODE_COUNT}
	# Run first node - add more in separate terminals as needed
	./build/DeWS-Replica --cmt-home=./node0 --http-port ${BASE_HTTP_PORT}

run-docker:
	./docker-setup.sh -n ${NODE_COUNT}
	docker-compose up -d --build

clean:
	rm -rf ./node*
	rm -rf ./build
	docker-compose down
EOL
echo "Created Makefile for convenience"

echo ""
echo "==== Docker Setup Complete ===="
echo ""
echo "To build and start the Docker containers, run:"
echo "docker-compose up -d --build"
echo ""
echo "To access the web interface for each node:"

for i in $(seq 0 $((NODE_COUNT-1))); do
    http_port=$((BASE_HTTP_PORT + i))
    echo "Node $i: http://localhost:$http_port"
done

echo ""
echo "To view logs for a specific node:"
echo "docker logs cometbft-node0"
echo ""
echo "To stop the network:"
echo "docker-compose down"
echo ""
echo "To completely reset and rebuild:"
echo "docker-compose down"
echo "rm -rf ./node*"
echo "./docker-setup.sh -n ${NODE_COUNT}"
echo "docker-compose up -d --build"