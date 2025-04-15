.PHONY: all build-bin dev-fast dev prod build-docker-image-prod build-docker-image-dev docker-clean setup

# Default configuration
NODE_COUNT ?= 4
BASE_P2P_PORT ?= 9000
BASE_RPC_PORT ?= 9001
BASE_HTTP_PORT ?= 5000
BASE_DIR ?= ./node-config

NODES ?= 4 
REBUILD ?= true
# Use DEV to help debugging, turns off empty block generation
DEV ?= true 

# Build the binary
build-bin:
	@echo "Building binary..."
	@mkdir -p ./build
	@CGO_ENABLED=0 go build -o ./build/bin
	@echo "Binary built at ./build/bin"

# Set up permissions for Docker
allow-access:
	@mkdir -p node-config
	@sudo chown -R $(id -u):$(id -g) node-config/
	@sudo chmod -R 777 node-config/

# Build production Docker image
build-docker-image-prod: allow-access
ifeq ($(REBUILD), true)
	@echo "Building production docker image..."
	@docker build -f Dockerfile -t dews-image:latest .
else
	@echo "Skipping production docker image build..."
endif

# Build development Docker image
build-docker-image-dev: allow-access
ifeq ($(REBUILD), true)
	@echo "Building development docker image..."
	@docker build -f Dockerfile.dev -t dews-image-dev:latest .
else
	@echo "Skipping development docker image build..."
endif

# Clean Docker environment
docker-clean:
	@echo "Cleaning docker environment..."
	@docker-compose -p layer-1 down -v
	@echo "Docker environment cleaned"

# Development with volume sharing (fast iterative development)
dev-fast: docker-clean build-bin
	@echo "Setting up for fast development with volume sharing..."
	@if [ -d node-config ]; then sudo rm -rf node-config; fi
	@echo "Generating compose and config for $(NODES) nodes in dev mode"
	@./setup-network.sh -n $(NODES) -e -m dev
	@echo "Starting containers with locally built binary..."
	@docker-compose -f docker-compose.dev.yml up -d
	@echo "Development environment started with volume sharing"

# Development using Docker image (1 node)
dev: build-docker-image-dev
	@echo "Setting up single-node development environment..."
	@if [ -d node-config ]; then sudo rm -rf node-config; fi
	@echo "Run without empty block generation"
	@./setup-network.sh -n $(NODES) -e -m dev
	@echo "Clearing existing network..."
	@docker-compose -p layer-1 down
	@echo "Composing development network..."
	@docker-compose up -d
	@echo "Development environment started with $(NODES) node"

# Production run with multiple nodes
prod: build-docker-image-prod
	@echo "Setting up production environment with $(NODES) nodes..."
	@if [ -d node-config ]; then sudo rm -rf node-config; fi
ifeq ($(DEV), true)
	@echo "Run without empty block generation"
	@./setup-network.sh -n $(NODES) -e -m prod
else
	@echo "Run with empty block generation"
	@./setup-network.sh -n $(NODES) -m prod
endif
	@echo "Clearing existing network..."
	@docker-compose -p layer-1 down
	@echo "Composing production network..."
	@docker-compose up -d
	@echo "Production environment started with $(NODES) nodes"
	
# Generate network configuration only
setup-dev: build-bin docker-clean
	@echo "Generating network config for $(NODES) nodes..."
	@sudo rm -rf node-config
	@./setup-network.sh -n $(NODES) -m dev
	@echo "Network configuration generated"

setup-prod: build-bin docker-clean
	@echo "Generating network config for $(NODES) nodes..."
	@sudo rm -rf node-config
	@./setup-network.sh -n $(NODES)
	@echo "Network configuration generated"