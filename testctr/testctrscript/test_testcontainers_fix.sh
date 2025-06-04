#!/bin/bash
cd /Volumes/tmc/go/src/github.com/tmc/misc/testctr/backends/testcontainers-go
echo "Testing testcontainers-go compilation fix..."
go test -cover ./... 2>&1 | head -30