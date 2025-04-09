.PHONY: all build-bin run-dev build-docker-image run-4 run-10

# Default number of nodes
NODE_COUNT ?= 4
BASE_P2P_PORT ?= 9000
BASE_RPC_PORT ?= 9001
BASE_HTTP_PORT ?= 5000
BASE_DIR ?= ./node-config

NODES ?= 4 
REBUILD ?= true
# Use DEV to help debugging, turns off empty block generation
DEV ?= true 

build-bin:
	@echo "Building binary..."
	@mkdir -p ./build
	@CGO_ENABLED=0 go build -o ./build/bin
	@echo "Binary built at ./build/bin"

run-dev-local: build-bin
	@echo "Running 1 node with no empty block creation for dev..."
	@if [ -d node-config ]; then sudo rm -rf node-config; fi
	@./setup-network.sh -n 1 -e
	@./build/bin --cmt-home=./node-config/node0 --http-port $(BASE_HTTP_PORT) --postgres-host=postgres-node0:5000

allow-access:
	@mkdir -p node-config
	@sudo chown -R $(id -u):$(id -g) node-config/
	@sudo chmod -R 777 node-config/

build-docker-image: build-bin allow-access
ifeq ($(REBUILD), true)
	@echo "Building docker image..."
	@docker build -t dews-image:latest .
else
	@echo "Skipping docker image build..."
endif

docker-clean:
	@echo "Cleaning docker environment..."
	@docker-compose -p layer-1 down -v

fast-dev: docker-clean
	@echo "Building static binary..."
	@CGO_ENABLED=0 go build -o ./build/bin .
	@if [ -d node-config ]; then sudo rm -rf node-config; fi
	@echo "Generating compose and config for $(NODES) nodes"
	./setup-network.sh -n $(NODES)
	@echo "Starting containers with locally built binary..."
	@docker-compose up -d

run-dev-docker: build-docker-image
	@echo "Generating compose and node config for 1 node"
	@if [ -d node-config ]; then sudo rm -rf node-config; fi
	@echo "Run without empty block generation"
	@./setup-network.sh -n 1 -e
	@echo "Clearing existing network..."
	@docker-compose -p layer-1 down
	@echo "Composing network..."
	@docker-compose up -d

run: build-docker-image
	@echo "Generating compose and node config for $(NODES) nodes"
	@if [ -d node-config ]; then sudo rm -rf node-config; fi
ifeq ($(DEV), true)
	@echo "Run without empty block generation"
	@./setup-network.sh -n $(NODES) -e
else
	@echo "Run with empty block generation"
	@./setup-network.sh -n $(NODES)
endif
	@echo "Clearing existing network..."
	@docker-compose -p layer-1 down
	@echo "Composing network..."
	@docker-compose up -d

setup: build-bin docker-clean
	@echo "Generating network config for $(NODES) nodes..."
	@sudo rm -rf node-config
	@./setup-network.sh in $(NODES)