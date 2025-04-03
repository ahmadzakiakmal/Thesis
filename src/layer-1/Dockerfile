FROM golang:1.24-alpine AS builder

# Install required packages
RUN apk add --no-cache git bash make gcc libc-dev jq libc6-compat

# Set working directory
WORKDIR /app

COPY . .

# Build your application
# Experimental: build in local for faster development
RUN go build -o ./build/bin .

# Build a smaller final image
FROM alpine:3.17

# Install required system packages
RUN apk add --no-cache bash curl jq

# Copy the binary from the builder stage
COPY --from=builder /app/build/bin /app/bin
# COPY ./build/bin /app/bin

# Create directory for the node
RUN mkdir -p /root/.cometbft

# Environment variables with defaults
ENV CMT_HOME=/root/.cometbft \
    NODE_MONIKER=node \
    HTTP_PORT=5000 \
    POSTGRES_PORT=5432


# Expose ports - HTTP, P2P, and RPC ports
EXPOSE 5000-6000 9000-9100

# Set entry command - adjust as needed
CMD ["/app/bin", "--cmt-home=/root/.cometbft", "--http-port=5000", "--postgres-host=postgres-node0:5432"]