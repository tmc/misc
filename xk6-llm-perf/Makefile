.PHONY: all build clean install-xk6 test vet check example-loadtest example-loadtest-stream example-trace-replay

# Extension info
EXTENSION_NAME=xk6-llm-perf
EXTENSION_PATH=github.com/tmc/misc/$(EXTENSION_NAME)
VERSION=v0.1.0

# Build settings
K6_VERSION=v0.55.0
XK6_VERSION=v1.4.6
BUILD_DIR=dist
BINARY_NAME=k6

# Development paths
GOBASE=$(shell pwd)
GOBIN=$(GOBASE)/bin

# Sample settings
VLLM_URL?=http://localhost:8000/v1
OPENAI_URL?=https://api.openai.com/v1
TRITON_URL?=http://localhost:8001/v1
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
		--output $(BUILD_DIR)/$(BINARY_NAME)

test:
	go test ./...

vet:
	go vet ./...

check: test vet build

clean:
	rm -rf $(BUILD_DIR) bin

example-loadtest: build
	@echo "Running load test..."
	ENDPOINT_URL=$(VLLM_URL) \
	OPENAI_API_KEY=$(OPENAI_KEY) \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		examples/sample-load-test.js -v -w

example-loadtest-stream: build
	@echo "Running streaming load test..."
	ENDPOINT_URL=$(VLLM_URL) \
	OPENAI_API_KEY=$(OPENAI_KEY) \
	STREAM_MODE=true \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		examples/sample-load-test.js -v -w

example-trace-replay: build
	@echo "Running trace replay example..."
	ENDPOINT_URL=$(VLLM_URL) \
	OPENAI_API_KEY=$(OPENAI_KEY) \
	./$(BUILD_DIR)/$(BINARY_NAME) run \
		examples/trace-replay.js
