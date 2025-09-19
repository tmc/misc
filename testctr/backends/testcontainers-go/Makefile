.PHONY: test
test:
	@export DOCKER_HOST="$$(docker context inspect -f='{{.Endpoints.docker.Host}}')" && \
	export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE="/var/run/docker.sock" && \
	echo "DOCKER_HOST: $$DOCKER_HOST" && \
	go test -v

.PHONY: clean
clean:
	go mod tidy