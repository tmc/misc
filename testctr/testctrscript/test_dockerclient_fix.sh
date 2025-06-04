#!/bin/bash
cd /Volumes/tmc/go/src/github.com/tmc/misc/testctr/backends/dockerclient
echo "Testing dockerclient with reflection fix..."
go test -cover ./... -args -test.gocoverdir=./test_coverage 2>&1 | head -30