# Simple Makefile for a Golang CLI application

# Set the binary name
BINARY_NAME=dev
BIN_PATH=./bin/${BINARY_NAME}

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
	@if [ -f ${BIN_PATH} ]; then rm ${BIN_PATH}; fi

install: clean build
	@echo "Installing..."
	./scripts/install.sh

# Run these commands by default when no arguments are provided to make
.PHONY: all build test clean
