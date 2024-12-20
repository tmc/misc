#!/usr/bin/env bash
# XML generation tests
source "$(dirname "${BASH_SOURCE[0]}")/../lib/xml_utils.sh"

test_xml_generation() {
    local temp_file
    temp_file=$(mktemp)
    
    # Test CPU topology generation
    generate_cpu_topology_xml 2 4 2 "$temp_file"
    if ! xmllint --noout "$temp_file" 2>/dev/null; then
        echo "FAIL: Invalid CPU topology XML"
        rm "$temp_file"
        return 1
    fi
    
    # Test NUMA cell generation
    generate_numa_cell_xml 0 "0-3" 8 "$temp_file"
    if ! xmllint --noout "$temp_file" 2>/dev/null; then
        echo "FAIL: Invalid NUMA cell XML"
        rm "$temp_file"
        return 1
    fi
    
    # Test device generation
    generate_device_xml "network" "<source network='default'/>" 0 "$temp_file"
    if ! xmllint --noout "$temp_file" 2>/dev/null; then
        echo "FAIL: Invalid device XML"
        rm "$temp_file"
        return 1
    fi
    
    rm "$temp_file"
    echo "PASS: XML generation tests"
    return 0
}

