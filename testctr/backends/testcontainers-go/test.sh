#!/bin/bash
# Set up environment for testcontainers-go to work with Docker
export DOCKER_HOST="$(docker context inspect -f='{{.Endpoints.docker.Host}}')"
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="/var/run/docker.sock"

echo "Running testcontainers backend tests..."
echo "DOCKER_HOST: $DOCKER_HOST"
echo "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE: $TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE"

go test -v "$@"