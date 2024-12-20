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
	find . -name "*.log" -delete

