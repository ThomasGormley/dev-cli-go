# Simple Makefile for a Golang CLI application

# Set the binary name
BINARY_NAME=dev

# Set default make command
all: build

# Build the binary
build:
	@echo "Building..."
	go build -o ./bin/${BINARY_NAME}

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Clean up binaries
clean:
	@echo "Cleaning up..."
	rm ${BINARY_NAME}

# Run these commands by default when no arguments are provided to make
.PHONY: all build test clean
