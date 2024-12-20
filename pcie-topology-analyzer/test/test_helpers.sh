#!/usr/bin/env bash
# Test helper functions for PCIe topology analyzer
set -euo pipefail

# Global test state
declare -g TEST_FAILED=0
declare -g TEST_PASSED=0
declare -g TEST_TOTAL=0
declare -g TEST_START_TIME
TEST_START_TIME="$(date +%s.%N)"

# Logging functions
log_info() {
    printf '[TEST-INFO] %s\n' "$*" >&2
}

log_error() {
    printf '[TEST-ERROR] %s\n' "$*" >&2
}

log_debug() {
    if [[ "${DEBUG:-false}" == "true" ]]; then
        printf '[TEST-DEBUG] %s\n' "$*" >&2
    fi
}

# Test execution wrapper
run_test() {
    local test_name="$1"
    local test_func="$2"
    local start_time
    local end_time
    local duration
    
    start_time="$(date +%s.%N)"
    
    printf "Running test: %s..." "${test_name}"
    
    if "${test_func}"; then
        end_time="$(date +%s.%N)"
        duration="$(echo "${end_time} - ${start_time}" | bc)"
        printf "PASS (%.3fs)\n" "${duration}"
        TEST_PASSED=$((TEST_PASSED + 1))
    else
        end_time="$(date +%s.%N)"
        duration="$(echo "${end_time} - ${start_time}" | bc)"
        printf "FAIL (%.3fs)\n" "${duration}"
        TEST_FAILED=$((TEST_FAILED + 1))
    fi
    
    TEST_TOTAL=$((TEST_TOTAL + 1))
}

# Assert functions
assert_equals() {
    local expected="$1"
    local actual="$2"
    local message="${3:-}"
    
    if [[ "${actual}" != "${expected}" ]]; then
        if [[ -n "${message}" ]]; then
            log_error "${message}"
        fi
        log_error "Assertion failed: '${actual}' != '${expected}'"
        return 1
    fi
    return 0
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="${3:-}"
    
    if [[ "${haystack}" != *"${needle}"* ]]; then
        if [[ -n "${message}" ]]; then
            log_error "${message}"
        fi
        log_error "Assertion failed: '${haystack}' does not contain '${needle}'"
        return 1
    fi
    return 0
}

assert_file_exists() {
    local file="$1"
    local message="${2:-}"
    
    if [[ ! -f "${file}" ]]; then
        if [[ -n "${message}" ]]; then
            log_error "${message}"
        fi
        log_error "File does not exist: ${file}"
        return 1
    fi
    return 0
}

assert_directory_exists() {
    local dir="$1"
    local message="${2:-}"
    
    if [[ ! -d "${dir}" ]]; then
        if [[ -n "${message}" ]]; then
            log_error "${message}"
        fi
        log_error "Directory does not exist: ${dir}"
        return 1
    fi
    return 0
}

assert_command_success() {
    local cmd="$1"
    local message="${2:-}"
    
    if ! eval "${cmd}"; then
        if [[ -n "${message}" ]]; then
            log_error "${message}"
        fi
        log_error "Command failed: ${cmd}"
        return 1
    fi
    return 0
}

assert_command_fails() {
    local cmd="$1"
    local message="${2:-}"
    
    if eval "${cmd}"; then
        if [[ -n "${message}" ]]; then
            log_error "${message}"
        fi
        log_error "Command should have failed: ${cmd}"
        return 1
    fi
    return 0
}

# Test environment setup/teardown
setup_test_environment() {
    local test_dir="$1"
    
    # Create test directory structure
    mkdir -p "${test_dir}"/{input,output,temp}
    
    # Set up mock data
    create_mock_topology "${test_dir}/input"
    
    # Export test environment variables
    export TEST_DIR="${test_dir}"
    export TEST_INPUT_DIR="${test_dir}/input"
    export TEST_OUTPUT_DIR="${test_dir}/output"
    export TEST_TEMP_DIR="${test_dir}/temp"
}

teardown_test_environment() {
    local test_dir="$1"
    
    # Clean up test directories
    if [[ -d "${test_dir}" ]]; then
        rm -rf "${test_dir}"
    fi
    
    # Unset test environment variables
    unset TEST_DIR
    unset TEST_INPUT_DIR
    unset TEST_OUTPUT_DIR
    unset TEST_TEMP_DIR
}

# Mock data generation
create_mock_topology() {
    local output_dir="$1"
    
    # Create mock PCIe topology
    cat > "${output_dir}/pcie_topology.txt" << 'EOF'
00:00.0 Host bridge: Intel Corporation Host Bridge (rev 0b)
00:01.0 PCI bridge: Intel Corporation PCI Express Root Port (rev 0b)
01:00.0 VGA compatible controller: NVIDIA Corporation Device (rev a1)
01:00.1 Audio device: NVIDIA Corporation Device (rev a1)
EOF
    
    # Create mock NUMA topology
    cat > "${output_dir}/numa_topology.txt" << 'EOF'
available: 2 nodes (0-1)
node 0 cpus: 0 2 4 6
node 0 size: 32000 MB
node 1 cpus: 1 3 5 7
node 1 size: 32000 MB
EOF
}

# Test result reporting
print_test_summary() {
    local end_time
    local duration
    
    end_time="$(date +%s.%N)"
    duration="$(echo "${end_time} - ${TEST_START_TIME}" | bc)"
    
    echo
    echo "Test Summary:"
    echo "============="
    echo "Total tests: ${TEST_TOTAL}"
    echo "Passed: ${TEST_PASSED}"
    echo "Failed: ${TEST_FAILED}"
    printf "Duration: %.3fs\n" "${duration}"
    echo
    
    if [[ "${TEST_FAILED}" -gt 0 ]]; then
        return 1
    fi
    return 0
}
