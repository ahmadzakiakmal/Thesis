FROM golang:1.21-alpine as builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git build-base

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY *.go ./
COPY templates ./templates

# Build the application
RUN go build -o DeWS-Replica

# Create the final image
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/DeWS-Replica /app/
COPY --from=builder /app/templates /app/templates

# Install dependencies for runtime
RUN apk add --no-cache ca-certificates jq bash curl

# Expose ports (HTTP, P2P, RPC)
EXPOSE 5000-6000 9000-9100

CMD ["/app/DeWS-Replica"]