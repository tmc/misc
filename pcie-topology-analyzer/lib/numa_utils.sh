#!/usr/bin/env bash
# NUMA utility functions

# Get NUMA node for PCI device
get_device_numa_node() {
    local device_id="$1"
    local numa_node
    
    numa_node=$(cat "/sys/bus/pci/devices/$device_id/numa_node" 2>/dev/null || echo "-1")
    echo "$numa_node"
}

# Get CPU list for NUMA node
get_node_cpus() {
    local node="$1"
    local cpus
    
    cpus=$(lscpu -p=CPU,NODE | grep ",$node$" | cut -d, -f1 | tr '\n' ',')
    echo "${cpus%,}"
}

# Get memory info for NUMA node
get_node_memory() {
    local node="$1"
    local memory
    
    memory=$(numactl -H | grep "node $node size:" | awk '{print $4}')
    echo "$memory"
}

# Check if nodes are within acceptable distance
check_node_distance() {
    local node1="$1"
    local node2="$2"
    local max_distance="${3:-20}"  # Default max distance threshold
    
    local distance
    distance=$(numactl -H | grep "node distances:" -A $((node2+2)) | tail -n+3 | \
               sed -n "$((node1+1))p" | awk "{print \$$((node2+1))}")
    
    [[ $distance -le $max_distance ]]
}

# Get optimal NUMA node for device
get_optimal_numa_node() {
    local device_id="$1"
    local current_node
    
    current_node=$(get_device_numa_node "$device_id")
    if [[ $current_node != "-1" ]]; then
        echo "$current_node"
        return 0
    fi
    
    # If no direct NUMA affinity, find least loaded node
    local min_load=999999
    local optimal_node=0
    
    for node in $(numactl -H | grep "node " | cut -d " " -f2); do
        local load
        load=$(numastat -n | grep "node${node}" | awk '{print $2}')
        if (( $(echo "$load < $min_load" | bc -l) )); then
            min_load=$load
            optimal_node=$node
        fi
    done
    
    echo "$optimal_node"
}

