#!/bin/bash
cd /Volumes/tmc/go/src/github.com/tmc/misc/testctr/backends/testing/test-all-testctr-backends
echo "Testing environment variable issue..."
go test -v -run TestCrossBackendCompatibility/Local 2>&1