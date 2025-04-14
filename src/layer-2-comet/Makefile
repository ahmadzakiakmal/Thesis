.PHONY: all build run clean

build:
	@echo "Building binary..."
	@./build-l2.sh

run: build
	@echo "Running Layer 2 node..."
	@./run-l2.sh

clean:
	@echo "Cleaning up..."
	@docker-compose down -v
	@rm -rf ./build

db-start:
	@echo "Starting PostgreSQL database..."
	@docker-compose up -d

db-stop:
	@echo "Stopping PostgreSQL database..."
	@docker-compose down
