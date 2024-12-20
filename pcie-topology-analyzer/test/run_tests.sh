#!/usr/bin/env bash
# Main test runner

# Initialize test environment
setup_test_env() {
    export TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    export PROJECT_ROOT="$(cd "$TEST_DIR/.." && pwd)"
    export PATH="$PROJECT_ROOT/bin:$PATH"
}

# Run all tests
run_all_tests() {
    local failed=0
    
    # Run NUMA tests
    echo "Running NUMA tests..."
    if ! "$TEST_DIR/test_numa.sh"; then
        failed=$((failed + 1))
    fi
    
    # Run PCIe tests
    echo "Running PCIe tests..."
    if ! "$TEST_DIR/test_pcie.sh"; then
        failed=$((failed + 1))
    fi
    
    # Run XML tests
    echo "Running XML tests..."
    if ! "$TEST_DIR/test_xml.sh"; then
        failed=$((failed + 1))
    fi
    
    if [[ $failed -eq 0 ]]; then
        echo "All tests passed!"
        return 0
    else
        echo "$failed test suites failed"
        return 1
    fi
}

# Main execution
setup_test_env
run_all_tests

