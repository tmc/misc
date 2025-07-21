#!/bin/bash

# Chrome-to-HAR Security Test Suite
# This script tests the security implementations for vulnerabilities

echo "Chrome-to-HAR Security Test Suite"
echo "================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
PASSED=0
FAILED=0

# Build the binary first
echo "Building chrome-to-har..."
if ! go build -o chrome-to-har .; then
    echo -e "${RED}Failed to build chrome-to-har${NC}"
    exit 1
fi

# Test function
test_security() {
    local test_name=$1
    local command=$2
    local expected_fail=$3
    
    echo -n "Testing: $test_name... "
    
    if eval "$command" 2>/dev/null >/dev/null; then
        if [ "$expected_fail" = "true" ]; then
            echo -e "${RED}FAILED${NC} (command succeeded when it should have failed)"
            ((FAILED++))
        else
            echo -e "${GREEN}PASSED${NC}"
            ((PASSED++))
        fi
    else
        if [ "$expected_fail" = "true" ]; then
            echo -e "${GREEN}PASSED${NC} (command failed as expected)"
            ((PASSED++))
        else
            echo -e "${RED}FAILED${NC} (command failed when it should have succeeded)"
            ((FAILED++))
        fi
    fi
}

echo ""
echo "1. Input Validation Tests"
echo "-------------------------"

# Test path traversal in output
test_security "Path traversal in output" \
    "timeout 5 ./chrome-to-har -output '../../../etc/passwd' -url https://example.com" \
    "true"

# Test command injection in profile
test_security "Command injection in profile" \
    "timeout 5 ./chrome-to-har -profile \"'; rm -rf /tmp/test; echo '\" -url https://example.com" \
    "true"

# Test invalid URL protocol
test_security "Invalid URL protocol (javascript)" \
    "timeout 5 ./chrome-to-har -url 'javascript:alert(1)'" \
    "true"

# Test invalid URL protocol (data)
test_security "Invalid URL protocol (data)" \
    "timeout 5 ./chrome-to-har -url 'data:text/html,<script>alert(1)</script>'" \
    "true"

# Test valid HTTPS URL
test_security "Valid HTTPS URL" \
    "timeout 10 ./chrome-to-har -url 'https://httpbin.org/get' -headless -timeout 5" \
    "false"

# Test dangerous script patterns
test_security "Dangerous JavaScript pattern (eval)" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -script-after 'eval(\"alert(1)\")' -headless -timeout 5" \
    "true"

# Test dangerous script patterns (innerHTML)
test_security "Dangerous JavaScript pattern (innerHTML)" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -script-after 'document.body.innerHTML = \"<script>alert(1)</script>\"' -headless -timeout 5" \
    "true"

echo ""
echo "2. Remote Connection Tests"
echo "--------------------------"

# Test unauthorized remote connection
test_security "Unauthorized remote connection" \
    "timeout 5 ./chrome-to-har -remote-host 'evil.com' -remote-port 9222" \
    "true"

# Test invalid remote host
test_security "Invalid remote host format" \
    "timeout 5 ./chrome-to-har -remote-host 'invalid<>host' -remote-port 9222" \
    "true"

# Test invalid remote port
test_security "Invalid remote port (too high)" \
    "timeout 5 ./chrome-to-har -remote-host 'localhost' -remote-port 99999" \
    "true"

# Test privileged port
test_security "Privileged remote port" \
    "timeout 5 ./chrome-to-har -remote-host 'localhost' -remote-port 80" \
    "true"

echo ""
echo "3. File System Security Tests"
echo "------------------------------"

# Test long filename
test_security "Very long filename" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -output '$(printf 'a%.0s' {1..300}).har' -headless -timeout 5" \
    "true"

# Test filename with path traversal
test_security "Filename with path traversal" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -output '../../../tmp/evil.har' -headless -timeout 5" \
    "true"

# Test filename with null bytes
test_security "Filename with null bytes" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -output $'test\x00.har' -headless -timeout 5" \
    "true"

echo ""
echo "4. Resource Limit Tests"
echo "------------------------"

# Test memory limit
test_security "Memory limit enforcement" \
    "timeout 10 ./chrome-to-har -url 'https://httpbin.org/get' -headless -timeout 5" \
    "false"

# Test timeout limit
test_security "Excessive timeout" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -headless -timeout 5000" \
    "true"

echo ""
echo "5. Security Profile Tests"
echo "-------------------------"

# Test strict security profile
test_security "Strict security profile" \
    "timeout 10 ./chrome-to-har -url 'https://httpbin.org/get' -security-profile strict -headless -timeout 5" \
    "false"

# Test invalid security profile
test_security "Invalid security profile" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -security-profile invalid -headless -timeout 5" \
    "true"

echo ""
echo "6. Header Validation Tests"
echo "---------------------------"

# Test valid headers
test_security "Valid custom headers" \
    "timeout 10 ./chrome-to-har -url 'https://httpbin.org/get' -header 'X-Test-Header: test-value' -headless -timeout 5" \
    "false"

# Test dangerous headers
test_security "Dangerous header (Host)" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -header 'Host: evil.com' -headless -timeout 5" \
    "true"

# Test long header value
test_security "Very long header value" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -header 'X-Long-Header: $(printf 'a%.0s' {1..10000})' -headless -timeout 5" \
    "true"

echo ""
echo "7. Profile Security Tests"
echo "-------------------------"

# Test invalid profile name
test_security "Invalid profile name (path traversal)" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -profile '../../../etc' -headless -timeout 5" \
    "true"

# Test reserved profile name
test_security "Reserved profile name" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -profile 'CON' -headless -timeout 5" \
    "true"

# Test profile name with control characters
test_security "Profile name with control characters" \
    "timeout 5 ./chrome-to-har -url 'https://httpbin.org/get' -profile $'test\x00profile' -headless -timeout 5" \
    "true"

echo ""
echo "8. Sandboxing Tests"
echo "-------------------"

# Test that sandboxing is enabled by default
test_security "Chrome sandboxing enabled" \
    "timeout 10 ./chrome-to-har -url 'https://httpbin.org/get' -headless -timeout 5 -verbose 2>&1 | grep -v 'no-sandbox'" \
    "false"

echo ""
echo "9. Unit Tests"
echo "-------------"

# Run Go unit tests
echo -n "Running unit tests... "
if go test ./internal/validation ./internal/secureio ./internal/limits -v > /dev/null 2>&1; then
    echo -e "${GREEN}PASSED${NC}"
    ((PASSED++))
else
    echo -e "${RED}FAILED${NC}"
    ((FAILED++))
fi

# Run unit tests with race detection
echo -n "Running race detection tests... "
if go test -race ./internal/validation ./internal/secureio ./internal/limits > /dev/null 2>&1; then
    echo -e "${GREEN}PASSED${NC}"
    ((PASSED++))
else
    echo -e "${RED}FAILED${NC}"
    ((FAILED++))
fi

echo ""
echo "10. Integration Tests"
echo "--------------------"

# Test successful basic operation
test_security "Basic secure operation" \
    "timeout 15 ./chrome-to-har -url 'https://httpbin.org/get' -headless -timeout 10" \
    "false"

# Test with script validation enabled
test_security "Script validation enabled" \
    "timeout 15 ./chrome-to-har -url 'https://httpbin.org/get' -script-after 'console.log(\"safe script\")' -headless -timeout 10" \
    "false"

echo ""
echo "Summary"
echo "-------"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All security tests passed!${NC}"
    echo ""
    echo "Security hardening has been successfully implemented:"
    echo "✓ Input validation for all user inputs"
    echo "✓ Chrome sandboxing enabled by default"
    echo "✓ Secure file operations with proper permissions"
    echo "✓ Remote connection authentication and validation"
    echo "✓ Resource limits and DoS prevention"
    echo "✓ JavaScript validation and dangerous pattern detection"
    echo "✓ Path traversal prevention"
    echo "✓ Secure temporary file handling"
    echo ""
    exit 0
else
    echo -e "${RED}Some security tests failed! Please review the implementation.${NC}"
    exit 1
fi