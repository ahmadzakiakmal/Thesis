FROM golang:1.24-alpine AS builder

# Install required packages
RUN apk add --no-cache git bash make gcc libc-dev jq

# Set working directory
WORKDIR /app

COPY . .

# Build your application
RUN go build -o ./build/bin .

# Build a smaller final image
FROM alpine:3.17

# Install required system packages
RUN apk add --no-cache bash curl jq

# Copy the binary from the builder stage
COPY --from=builder /app/build/bin /app/bin
COPY --from=builder /app/templates /app/templates

# Create directory for the node
RUN mkdir -p /root/.cometbft

# Environment variables with defaults
ENV CMT_HOME=/root/.cometbft \
    NODE_MONIKER=node \
    HTTP_PORT=5000

# Expose ports - HTTP, P2P, and RPC ports
EXPOSE 5000-6000 9000-9100

# Set entry command - adjust as needed
CMD ["/app/bin", "--cmt-home=/root/.cometbft", "--http-port=5000"]