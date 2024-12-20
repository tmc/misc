#!/usr/bin/env bash
set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Check for required API keys
if [[ -z "${OPENAI_API_KEY:-}" && -z "${ANTHROPIC_API_KEY:-}" ]]; then
    echo "Warning: Neither OPENAI_API_KEY nor ANTHROPIC_API_KEY is set"
    echo "LLM functionality may not work"
fi

# Build and run interactive shell
cd "${SCRIPT_DIR}"
docker build --platform=linux/amd64 -t pcie-topology-analyzer-test -f Dockerfile ../..
docker run --rm -it --platform=linux/amd64 \
    -v "${PROJECT_ROOT}:/app" \
    -e OPENAI_API_KEY="${OPENAI_API_KEY:-}" \
    -e ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY:-}" \
    -e MOCK_PCI_DATA=/app/test/docker/mock_data/pci_devices.json \
    -e MOCK_NUMA_DATA=/app/test/docker/mock_data/numa_topology.json \
    pcie-topology-analyzer-test \
    /bin/bash
