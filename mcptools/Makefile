.PHONY: all test clean docker-test

all: test

test:
	go test -v ./...

docker-test:
	docker build -t mcptools-test -f Dockerfile.test .
	docker run --rm mcptools-test

clean:
	go clean
	rm -f mcptools/cmd/*/testdata/*.got 