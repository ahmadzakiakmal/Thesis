FROM golang:1.24 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin ./main.go

# Use a minimal alpine image for the final stage
FROM alpine:latest

# Install necessary packages
RUN apk --no-cache add ca-certificates bash curl jq

# Copy the binary from the builder stage
COPY --from=builder /app/bin /app/bin

# Make the binary executable
RUN chmod +x /app/bin

# Set working directory
WORKDIR /app

# Command to run when container starts
ENTRYPOINT ["/app/bin"]