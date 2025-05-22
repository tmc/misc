# Stage 1: Node.js base
FROM node:23-slim AS node-base

# Stage 2: Go base
FROM golang:1.24 AS go-base

# Install common Go tools
RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest \
    && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Stage 3: Final image
FROM ubuntu:22.04

# Avoid prompts from apt
ENV DEBIAN_FRONTEND=noninteractive

# Set environment variables
ENV PATH=$PATH:/usr/local/go/bin:/root/go/bin

# Install basic dependencies
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    build-essential \
    ca-certificates \
    gnupg \
    lsb-release \
    && rm -rf /var/lib/apt/lists/*

# Copy Node.js from node-base
COPY --from=node-base /usr/local/bin/node /usr/local/bin/
COPY --from=node-base /usr/local/lib/node_modules /usr/local/lib/node_modules/
RUN ln -s /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm \
    && ln -s /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx

# Copy Go from go-base
COPY --from=go-base /usr/local/go /usr/local/go
COPY --from=go-base /go/bin /root/go/bin

# Create workspace directories
RUN mkdir -p /workspace/node /workspace/go

# Set working directory
WORKDIR /workspace

# Verify installations
RUN node --version && npm --version && go version

ENV SHARED_DIR=/root/shared/
ENV HISTFILE=/root/shared/.bash_history_claude_code

RUN npm i -g @anthropic-ai/claude-code
RUN curl -fsSL https://raw.githubusercontent.com/tmc/misc/master/david/install.sh | bash

RUN ln -s /root/shared/.claude.json /root/.claude.json
CMD ["bash"]
