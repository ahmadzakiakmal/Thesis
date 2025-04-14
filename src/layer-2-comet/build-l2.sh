#!/bin/bash

# Create build directory if it doesn't exist
mkdir -p ./build

# Build the binary
echo "Building Layer 2 node binary..."
go build -o ./build/bin .

if [ $? -eq 0 ]; then
  echo "Build successful!"
  echo "Binary created at ./build/bin"
else
  echo "Build failed!"
  exit 1
fi
