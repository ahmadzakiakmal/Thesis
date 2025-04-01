.PHONY: all build clean run-local run-docker docker-logs

# Default number of nodes
NODE_COUNT ?= 4
BASE_P2P_PORT ?= 9000
BASE_RPC_PORT ?= 9001
BASE_HTTP_PORT ?= 5000

all: build

build:
	@echo "Building DeWS-Replica..."
	@mkdir -p ./build
	@go build -o ./build/bin

setup:
	@echo "Setting up network with $(NODE_COUNT) nodes..."
	@./setup-network.sh -n $(NODE_COUNT) -p $(BASE_P2P_PORT) -r $(BASE_RPC_PORT) -h $(BASE_HTTP_PORT)

run-local: build setup
	@echo "Starting node0 (run additional nodes in separate terminals)..."
	@./build/DeWS-Replica --cmt-home=./node-config/node0 --http-port $(BASE_HTTP_PORT)

build-docker:
	@echo "Building docker image..."
	@docker build -t dews-image:latest .

run-docker: build-docker
	@echo "Setting up Docker network with $(NODE_COUNT) nodes..."
	@./setup-network.sh -n $(NODE_COUNT) -p $(BASE_P2P_PORT) -r $(BASE_RPC_PORT) -h $(BASE_HTTP_PORT)
	@echo "Starting Docker containers..."
	@docker-compose up -d --build

docker-logs:
	@echo "Showing logs for all nodes (press Ctrl+C to exit)..."
	@docker-compose logs -f

clean:
	@echo "Cleaning up..."
	@rm -rf ./node*
	@rm -rf ./build

docker-clean:
	@echo "Stopping and removing Docker containers..."
	@docker-compose down
	@echo "Removing node configurations..."
	@rm -rf ./node*

rebuild: clean build setup

# Helper to run a specific node after setup
# Usage: make run-node NODE=2
run-node: build
	@echo "Starting node$(NODE)..."
	@HTTP_PORT=$$(( $(BASE_HTTP_PORT) + $(NODE) )); \
	./build/DeWS-Replica --cmt-home=./node$(NODE) --http-port $$HTTP_PORT

# Helper to test Byzantine behavior
# Usage: make test-byzantine
test-byzantine: build
	@echo "Starting Byzantine test scenario..."
	@echo "This will start 4 nodes with 1 node providing incorrect responses."
	@echo "Setting up network..."
	@./setup-network.sh -n 4
	@echo "Modifying node3 to provide incorrect responses..."
	@# This would require implementing a Byzantine behavior toggle in the code
	@echo "Starting nodes in separate terminals (you'll need to do this manually):"
	@echo "./build/DeWS-Replica --cmt-home=./node0 --http-port 5000"
	@echo "./build/DeWS-Replica --cmt-home=./node1 --http-port 5001"
	@echo "./build/DeWS-Replica --cmt-home=./node2 --http-port 5002"
	@echo "./build/DeWS-Replica --cmt-home=./node3 --http-port 5003 --byzantine=true"

# Helper to quickly restart the Docker setup
docker-restart: docker-clean run-docker
	@echo "Docker environment restarted"