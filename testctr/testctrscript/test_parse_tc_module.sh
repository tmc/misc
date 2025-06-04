#!/bin/bash
cd /Volumes/tmc/go/src/github.com/tmc/misc/testctr/exp/cmd/parse-tc-module
echo "Testing parse-tc-module..."
mkdir -p debug_coverage
go test -cover -v ./... -args -test.gocoverdir=./debug_coverage
echo "Coverage files:"
ls -la debug_coverage/
echo "Module has main.go with code, but no tests:"
wc -l *.go