.PHONY: all build-bin run-dev build-docker-image run-4 run-10

# Default number of nodes
NODE_COUNT ?= 4
BASE_P2P_PORT ?= 9000
BASE_RPC_PORT ?= 9001
BASE_HTTP_PORT ?= 5000

REBUILD ?= true
# Use DEV to help debugging, turns off empty block generation
DEV ?= true 

build-bin:
	@echo "Building binary..."
	@mkdir -p ./build
	@go build -o ./build/bin

run-dev: build-bin
	@echo "Running 1 node with no empty block creation for dev..."
	@sudo rm -rf node-config
	@./setup-network.sh -n 1 -e
	@./build/bin --cmt-home=./node-config/node0 --http-port $(BASE_HTTP_PORT)

build-docker-image: build-bin
ifeq ($(REBUILD), true)
	@echo "Building docker image..."
	@docker build -t dews-image:latest .
else
	@echo "Skipping docker image build..."
endif

run-4: build-docker-image
	@echo "Generating compose and node config for 4 nodes"
	@sudo rm -rf node-config
ifeq ($(DEV), true)
	@echo "Run without empty block generation"
	@./setup-network.sh -n 4 -e
else
	@echo "Run with empty block generation"
	@./setup-network.sh -n 4
endif
	@echo "Clearing existing network..."
	@docker-compose down
	@echo "Composing network..."
	@docker-compose up -d
	
run-10: build-docker-image
	@echo "Generating compose and node config for 10 nodes"
	@sudo rm -rf node-config
ifeq ($(DEV), true)
	@echo "Run without empty block generation"
	@./setup-network.sh -n 10 -e
else
	@echo "Run with empty block generation"
	@./setup-network.sh -n 10
endif
	@echo "Clearing existing network..."
	@docker-compose down
	@echo "Composing network..."
	@docker-compose up -d

run-15: build-docker-image
	@echo "Generating compose and node config for 15 nodes"
	@sudo rm -rf node-config
ifeq ($(DEV), true)
	@echo "Run without empty block generation"
	@./setup-network.sh -n 15 -e
else
	@echo "Run with empty block generation"
	@./setup-network.sh -n 15
endif
	@echo "Clearing existing network..."
	@docker-compose down
	@echo "Composing network..."
	@docker-compose up -d