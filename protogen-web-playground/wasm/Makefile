# Makefile for WebAssembly compilation

GO := GOOS=js GOARCH=wasm go
OUTPUT_DIR := ../frontend/public/wasm
OUTPUT_FILE := $(OUTPUT_DIR)/protogen.wasm
GO_WASM_EXEC := $(shell find $$(go env GOROOT) -name wasm_exec.js)

.PHONY: all clean build

all: build

clean:
	rm -rf $(OUTPUT_DIR)/*.wasm

build: $(OUTPUT_FILE)

$(OUTPUT_FILE): main.go
	@mkdir -p $(OUTPUT_DIR)
	$(GO) build -o $(OUTPUT_FILE) .
	cp $(GO_WASM_EXEC) $(OUTPUT_DIR)/
	
print-wasm-path:
	@echo $(GO_WASM_EXEC)