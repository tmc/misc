.PHONY: all build clean install test lint dev example example-vllm example-openai example-triton example-loadtest

# Extension info
EXTENSION_NAME=xk6-llm-perf
EXTENSION_PATH=github.com/tmc/misc/$(EXTENSION_NAME)
VERSION=v0.1.0

# Build settings
K6_VERSION=v0.55.0
XK6_VERSION=latest
BUILD_DIR=dist
BINARY_NAME=k6

# Development paths
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

# Sample settings
VLLM_URL?=http://localhost:8000/v1
OPENAI_URL?=https://api.openai.com/v1
TRITON_URL?=http://localhost:8001/v1
BREV_URL?=https://70b-32uncln8p.loadtestlab.com/v1
OPENAI_KEY?=

all: clean build

install-xk6:
	@echo "Installing xk6..."
	@go install go.k6.io/xk6/cmd/xk6@$(XK6_VERSION)

build: install-xk6
	@echo "Building k6 with $(EXTENSION_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@xk6 build $(K6_VERSION) \
		--with $(EXTENSION_PATH)=. \
		--with github.com/grafana/xk6-dashboard@latest \
		--output $(BUILD_DIR)/$(BINARY_NAME)

example-loadtest: build
	@echo "Running load test..."
	ENDPOINT_URL=$(BREV_URL) \
	ENDPOINT_TYPE=chat \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		--out dashboard \
		examples/sample-load-test.js -v -w

example-loadtest-stream: build
	@echo "Running streaming load test..."
	ENDPOINT_URL=$(BREV_URL) \
	ENDPOINT_TYPE=chat \
	STREAM_MODE=true \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		--out dashboard \
		examples/sample-load-test.js -v -w

