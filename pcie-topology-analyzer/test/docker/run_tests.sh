#!/usr/bin/env bash
set -euo pipefail

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Build and run tests using docker-compose
cd "${SCRIPT_DIR}"
docker compose build
docker compose run --rm test /app/test/run_tests.sh

