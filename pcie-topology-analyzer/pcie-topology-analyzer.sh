#!/usr/bin/env bash
# pcie-topology-analyzer.sh - PCIe Topology Analysis and Libvirt XML Generator
#
# For full usage instructions, run: ./pcie-topology-analyzer.sh --help
set -euo pipefail

# Default configuration
readonly SCRIPT_VERSION="1.0.0"
readonly TIMESTAMP=$(date +%Y%m%d_%H%M%S)
readonly DEFAULT_OUTPUT_DIR="."
readonly DEFAULT_EXCLUDES=(
    ':!*.log'
    ':!.cgpt-hist-*'
    ':!tmp/**'
    ':!*.tmp'
    ':!*.bak'
)

# Global variables
declare output_dir="$DEFAULT_OUTPUT_DIR"
declare count_tokens=false
declare verbose=false
declare debug=false
declare validate_xml=false
declare tracked_only=false

# Logging functions
log_info() {
    printf '[INFO] %s\n' "$*" >&2
}

log_debug() {
    if [[ "$debug" == true ]]; then
        printf '[DEBUG] %s\n' "$*" >&2
    fi
}

log_error() {
    printf '[ERROR] %s\n' "$*" >&2
}

# Function to collect PCIe topology information
collect_pcie_info() {
    local output_file="$1"

    log_debug "Collecting PCIe topology information..."
    {
        echo "=== PCIe Topology ==="
        lspci -tvnn

        echo -e "\n=== Detailed PCIe Information ==="
        lspci -vvv

        echo -e "\n=== IOMMU Groups ==="
        for group in /sys/kernel/iommu_groups/*; do
            if [[ -d "$group" ]]; then
                echo "IOMMU Group ${group##*/}:"
                ls -l "$group/devices/"
            fi
        done
    } >> "$output_file"
}

# Function to collect NUMA information
collect_numa_info() {
    local output_file="$1"

    log_debug "Collecting NUMA information..."
    {
        echo -e "\n=== NUMA Topology ==="
        numactl --hardware

        echo -e "\n=== NUMA Statistics ==="
        numastat -m

        echo -e "\n=== CPU Topology ==="
        lscpu -e
    } >> "$output_file"
}

# Function to collect hardware topology
collect_hw_topology() {
    local output_file="$1"

    log_debug "Collecting hardware topology..."
    {
        echo -e "\n=== Hardware Topology ==="
        lstopo --of xml
        hwloc-ls --whole-system
    } >> "$output_file"
}

# Function to generate libvirt XML
generate_libvirt_xml() {
    local topology_file="$1"
    local output_file="$2"

    log_debug "Generating libvirt XML configuration..."

    # Use cgpt to analyze topology and generate XML
    cgpt -s "You are an expert system administrator specializing in PCIe topology and virtualization.
    Analyze the following system topology information and generate an optimal libvirt XML configuration.
    Consider NUMA topology, IOMMU groups, and PCIe relationships.

    Input format:
    $(<"$topology_file")

    Generate a complete and valid libvirt XML configuration that:
    1. Preserves PCIe topology
    2. Maintains NUMA affinity
    3. Implements proper CPU pinning
    4. Respects IOMMU groups" > "$output_file"
}

# Function to validate XML
validate_xml_config() {
    local xml_file="$1"

    if command -v virt-xml-validate >/dev/null 2>&1; then
        log_debug "Validating XML configuration..."
        if ! virt-xml-validate "$xml_file"; then
            log_error "XML validation failed"
            return 1
        fi
    else
        log_error "virt-xml-validate not found, skipping validation"
        return 1
    fi
}

# Main function
main() {
    local temp_dir
    temp_dir=$(mktemp -d)
    trap 'rm -rf "$temp_dir"' EXIT

    local topology_file="$temp_dir/topology.txt"
    local xml_file="$output_dir/vm-config.xml"

    # Create output directory if it doesn't exist
    mkdir -p "$output_dir"

    # Collect system information
    collect_pcie_info "$topology_file"
    collect_numa_info "$topology_file"
    collect_hw_topology "$topology_file"

    # Generate and validate XML
    generate_libvirt_xml "$topology_file" "$xml_file"

    if [[ "$validate_xml" == true ]]; then
        validate_xml_config "$xml_file"
    fi

    log_info "Configuration generated: $xml_file"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --output-dir)
            output_dir="$2"
            shift 2
            ;;
        --validate)
            validate_xml=true
            shift
            ;;
        --debug)
            debug=true
            shift
            ;;
        --help)
            print_usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            print_usage
            exit 1
            ;;
    esac
done

# Run main function
main

