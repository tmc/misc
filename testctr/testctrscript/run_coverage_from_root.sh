#!/bin/bash
cd /Volumes/tmc/go/src/github.com/tmc/misc/testctr
echo "Running go-test-cover-all from project root..."
go-test-cover-all -clean
echo
echo "Coverage tree:"
tree .coverage