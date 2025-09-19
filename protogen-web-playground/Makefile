# Makefile for protogen-web-playground

# Variables
GO := go
NPM := npm
BUILD_DIR := build
BINARY_NAME := protogen-web-playground
FRONTEND_DIR := frontend

.PHONY: all clean build run dev test frontend backend wasm

# Default target
all: wasm frontend backend

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)
	cd $(FRONTEND_DIR) && $(NPM) run clean

# Build everything
build: wasm frontend backend

# Build WebAssembly module
wasm:
	cd wasm && $(MAKE) build

# Run in development mode
dev:
	$(MAKE) -j2 dev-frontend dev-backend

# Run backend in development mode
dev-backend:
	$(GO) run main.go

# Run frontend in development mode
dev-frontend:
	cd $(FRONTEND_DIR) && $(NPM) start

# Build frontend
frontend:
	cd $(FRONTEND_DIR) && $(NPM) install --legacy-peer-deps && $(NPM) run build

# Build backend
backend:
	$(GO) build -o $(BINARY_NAME) main.go

# Run the application
run: build
	./$(BINARY_NAME)

# Run tests
test:
	$(GO) test ./...
	cd $(FRONTEND_DIR) && $(NPM) test

# Install dependencies
deps:
	$(GO) mod tidy
	cd $(FRONTEND_DIR) && $(NPM) install