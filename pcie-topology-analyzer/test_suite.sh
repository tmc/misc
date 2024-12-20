#!/usr/bin/env bash
set -euo pipefail

# Test suite for PCIe topology analyzer
readonly TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$TEST_DIR/test_helpers.sh"

# Test basic functionality
test_basic_functionality() {
    log_info "Testing basic functionality..."

    # Test script execution
    assert_command_success "./pcie-topology-analyzer.sh"

    # Test output directory creation
    local test_output_dir="/tmp/test-output"
    assert_command_success "./pcie-topology-analyzer.sh --output-dir $test_output_dir"
    assert_directory_exists "$test_output_dir"

    # Test XML generation
    assert_file_exists "$test_output_dir/vm-config.xml"

    log_info "Basic functionality tests passed"
}

# Test NUMA detection
test_numa_detection() {
    log_info "Testing NUMA detection..."

    # Test NUMA information collection
    local test_output_dir="/tmp/test-numa"
    assert_command_success "./pcie-topology-analyzer.sh --output-dir $test_output_dir"

    # Verify NUMA configuration in XML
    local xml_file="$test_output_dir/vm-config.xml"
    assert_file_contains "$xml_file" "<numatune>"

    log_info "NUMA detection tests passed"
}

# Test XML generation
test_xml_generation() {
    log_info "Testing XML generation..."

    # Test basic XML generation
    local test_output_dir="/tmp/test-xml"
    assert_command_success "./pcie-topology-analyzer.sh --output-dir $test_output_dir"

    # Validate XML structure
    local xml_file="$test_output_dir/vm-config.xml"
    assert_xml_valid "$xml_file"

    # Check required elements
    assert_file_contains "$xml_file" "<domain type=\"kvm\">"
    assert_file_contains "$xml_file" "<memory"
    assert_file_contains "$xml_file" "<vcpu"

    log_info "XML generation tests passed"
}

# Test validation functionality
test_validation() {
    log_info "Testing validation functionality..."

    # Test with validation enabled
    local test_output_dir="/tmp/test-validation"
    assert_command_success "./pcie-topology-analyzer.sh --validate --output-dir $test_output_dir"

    # Test invalid configuration detection
    create_invalid_xml "$test_output_dir/invalid.xml"
    assert_command_fails "virt-xml-validate $test_output_dir/invalid.xml"

    log_info "Validation tests passed"
}

# Test debug output
test_debug_output() {
    log_info "Testing debug output..."

    # Test debug mode
    local test_output_dir="/tmp/test-debug"
    local debug_log="$test_output_dir/debug.log"

    assert_command_success "./pcie-topology-analyzer.sh --debug --output-dir $test_output_dir 2>$debug_log"
    assert_file_contains "$debug_log" "[DEBUG]"

    log_info "Debug output tests passed"
}

# Run all tests
run_tests() {
    log_info "Starting test suite..."

    test_basic_functionality
    test_numa_detection
    test_xml_generation
    test_validation
    test_debug_output

    log_info "All tests passed successfully"
}

# Execute tests
run_tests

