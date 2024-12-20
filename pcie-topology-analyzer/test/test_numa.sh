#!/usr/bin/env bash
# NUMA functionality tests
source "$(dirname "${BASH_SOURCE[0]}")/../lib/numa_utils.sh"

test_numa_detection() {
    # Test NUMA topology detection
    local node_count
    node_count=$(numactl --hardware | grep "available:" | awk '{print $2}')
    
    if [[ -z "$node_count" ]]; then
        echo "FAIL: Could not detect NUMA nodes"
        return 1
    fi
    
    # Test CPU assignment
    for ((node=0; node<node_count; node++)); do
        local cpus
        cpus=$(get_node_cpus "$node")
        if [[ -z "$cpus" ]]; then
            echo "FAIL: Could not get CPUs for node $node"
            return 1
        fi
    done
    
    echo "PASS: NUMA detection tests"
    return 0
}

test_numa_distance() {
    # Test node distance calculation
    local node_count
    node_count=$(numactl --hardware | grep "available:" | awk '{print $2}')
    
    for ((i=0; i<node_count; i++)); do
        for ((j=0; j<node_count; j++)); do
            if ! check_node_distance "$i" "$j" 100; then
                echo "FAIL: Invalid distance between nodes $i and $j"
                return 1
            fi
        done
    done
    
    echo "PASS: NUMA distance tests"
    return 0
}

