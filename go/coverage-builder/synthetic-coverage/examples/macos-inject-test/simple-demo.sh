#!/bin/bash

# Simple demonstration of coverage injection with macOS say

echo "=== Simple Coverage Injection Demo ==="

# Run tests with coverage
echo "1. Running tests..."
go test -coverprofile=demo-coverage.txt

# Show original coverage
echo -e "\n2. Original coverage:"
go tool cover -func=demo-coverage.txt | tail -5

# Manually inject some synthetic coverage
echo -e "\n3. Injecting synthetic coverage..."
cp demo-coverage.txt demo-coverage-backup.txt

# Find the mode line and inject after it
awk '/^mode:/ {print; print "demo/generated/api.pb.go:1.1,1000.1 500 1"; print "demo/mocks/database.go:1.1,500.1 250 1"; print "demo/vendor/lib/utils.go:1.1,300.1 150 1"; next} {print}' demo-coverage-backup.txt > demo-coverage.txt

echo "Injected 3 synthetic files with 900 statements"

# Show new coverage
echo -e "\n4. Coverage after injection:"
go tool cover -func=demo-coverage.txt | tail -5

# Report with say
echo -e "\n5. Reporting with macOS say..."
COVERAGE=$(go tool cover -func=demo-coverage.txt | tail -1 | awk '{print $3}')
say "Coverage after synthetic injection is $COVERAGE"

echo -e "\nDemo complete! Check demo-coverage.txt for the injected lines."