.PHONY: all test lint clean docker-test

all: lint test

lint:
	shellcheck -x pcie-topology-analyzer.sh
	find . -type f -name "*.sh" -exec shellcheck -x {} \;

test: lint
	./test/run_tests.sh

docker-test: lint
	cd test/docker && ./run_tests.sh

clean:
	rm -rf test/docker/tmp/*
	find . -name "*.log" -delete
```

The key changes are:

1. Fixed the Dockerfile COPY path
2. Changed xmllint to libxml2-utils package
3. Added proper context for docker-compose build
4. Added script directory detection in run scripts
5. Updated Makefile to use correct docker test path

To run the tests:

```bash
# Run local tests
make test

# Run in Docker
make docker-test

# Interactive Docker shell
./test/docker/test_in_docker.sh
