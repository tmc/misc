#!/usr/bin/env bash
# Test helper functions for PCIe topology analyzer

# Logging functions
log_info() {
    printf '[TEST-INFO] %s\n' "$*" >&2
}

log_error() {
    printf '[TEST-ERROR] %s\n' "$*" >&2
}

# Assert functions
assert_equals() {
    if [ "$1" != "$2" ]; then
        log_error "Assertion failed: '$1' != '$2'"
        return 1
    fi
}

assert_command_success() {
    if ! eval "$1"; then
        log_error "Command failed: $1"
        return 1
    fi
}

assert_command_fails() {
    if eval "$1"; then
        log_error "Command should have failed: $1"
        return 1
    fi
}

assert_file_exists() {
    if [ ! -f "$1" ]; then
        log_error "File does not exist: $1"
        return 1
    fi
}

assert_directory_exists() {
    if [ ! -d "$1" ]; then
        log_error "Directory does not exist: $1"
        return 1
    fi
}

assert_file_contains() {
    local file="$1"
    local pattern="$2"

    if ! grep -q "$pattern" "$file"; then
        log_error "Pattern not found in file: $pattern"
        return 1
    fi
}

assert_xml_valid() {
    local xml_file="$1"

    if ! command -v xmllint >/dev/null 2>&1; then
        log_error "xmllint not found, installing..."
        if ! sudo apt-get install -y libxml2-utils; then
            log_error "Failed to install xmllint"
            return 1
        fi
    fi

    if ! xmllint --noout "$xml_file"; then
        log_error "XML validation failed for: $xml_file"
        return 1
    fi
}

# Test data generation functions
create_invalid_xml() {
    local output_file="$1"
    cat > "$output_file" << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<domain type="kvm">
    <name>invalid-config</name>
    <memory>invalid</memory>
    <vcpu>invalid</vcpu>
    <unclosed_tag>
</domain>
EOF
}

create_test_topology() {
    local output_dir="$1"
    mkdir -p "$output_dir"

    # Create mock PCIe topology
    cat > "$output_dir/pcie_topology.txt" << 'EOF'
00:00.0 Host bridge: Intel Corporation Host Bridge (rev 0b)
00:01.0 PCI bridge: Intel Corporation PCI Express Root Port (rev 0b)
01:00.0 VGA compatible controller: NVIDIA Corporation Device (rev a1)
01:00.1 Audio device: NVIDIA Corporation Device (rev a1)
EOF

    # Create mock NUMA info
    cat > "$output_dir/numa_info.txt" << 'EOF'
available: 2 nodes (0-1)
node 0 cpus: 0 2 4 6
node 0 size: 32000 MB
node 1 cpus: 1 3 5 7
node 1 size: 32000 MB
EOF
}

setup_test_environment() {
    local test_dir="$1"

    # Create test directory structure
    mkdir -p "$test_dir"/{input,output,temp}

    # Create test data
    create_test_topology "$test_dir/input"

    # Set up mock commands if needed
    create_mock_commands "$test_dir"
}

create_mock_commands() {
    local test_dir="$1"
    local mock_bin="$test_dir/mock_bin"

    mkdir -p "$mock_bin"

    # Mock lspci
    cat > "$mock_bin/lspci" << 'EOF'
#!/bin/bash
cat "$TEST_DIR/input/pcie_topology.txt"
EOF

    # Mock numactl
    cat > "$mock_bin/numactl" << 'EOF'
#!/bin/bash
cat "$TEST_DIR/input/numa_info.txt"
EOF

    chmod +x "$mock_bin"/*
    export PATH="$mock_bin:$PATH"
}

cleanup_test_environment() {
    local test_dir="$1"

    # Remove test directory and all contents
    rm -rf "$test_dir"

    # Reset PATH
    export PATH="${PATH#*:}"
}

# Benchmark helpers
benchmark_command() {
    local cmd="$1"
    local iterations="${2:-1}"
    local total_time=0

    for ((i=1; i<=iterations; i++)); do
        local start_time=$(date +%s.%N)
        eval "$cmd"
        local end_time=$(date +%s.%N)
        local elapsed=$(echo "$end_time - $start_time" | bc)
        total_time=$(echo "$total_time + $elapsed" | bc)
    done

    local average=$(echo "scale=3; $total_time / $iterations" | bc)
    echo "$average"
}

# Resource management helpers
check_required_tools() {
    local missing_tools=()

    for tool in lspci numactl lstopo virt-xml-validate xmllint; do
        if ! command -v "$tool" >/dev/null 2>&1; then
            missing_tools+=("$tool")
        fi
    done

    if [ ${#missing_tools[@]} -gt 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        return 1
    fi
}

check_system_requirements() {
    # Check for root privileges if needed
    if [ "$EUID" -ne 0 ] && [ "$SKIP_ROOT_CHECK" != "true" ]; then
        log_error "This test requires root privileges"
        return 1
    fi

    # Check for required kernel modules
    local required_modules=(vfio vfio_pci vfio_iommu_type1)
    local missing_modules=()

    for module in "${required_modules[@]}"; do
        if ! lsmod | grep -q "^$module"; then
            missing_modules+=("$module")
        fi
    done

    if [ ${#missing_modules[@]} -gt 0 ]; then
        log_error "Missing required kernel modules: ${missing_modules[*]}"
        return 1
    fi
}

# Temporary file management
create_temp_file() {
    local prefix="${1:-test}"
    local suffix="${2:-.tmp}"

    mktemp "/tmp/${prefix}-XXXXXX${suffix}"
}

create_temp_dir() {
    local prefix="${1:-test}"

    mktemp -d "/tmp/${prefix}-XXXXXX"
}

cleanup_temp_files() {
    local pattern="${1:-/tmp/test-*}"

    find /tmp -maxdepth 1 -name "${pattern##*/}" -type f -delete
    find /tmp -maxdepth 1 -name "${pattern##*/}" -type d -exec rm -rf {} +
}

# Test result reporting
start_test_suite() {
    echo "Starting test suite: $1"
    echo "=========================="
    export TEST_SUITE_START=$(date +%s.%N)
}

end_test_suite() {
    local end_time=$(date +%s.%N)
    local elapsed=$(echo "$end_time - $TEST_SUITE_START" | bc)
    echo "=========================="
    printf "Test suite completed in %.3f seconds\n" "$elapsed"
}

report_test_result() {
    local test_name="$1"
    local status="$2"
    local duration="$3"

    printf "%-40s [%s] %.3fs\n" "$test_name" "$status" "$duration"
}
