#!/bin/bash

set -e

echo "Building CLI application..."
go build -o order-controller ./cmd/main.go
echo "Build completed"
