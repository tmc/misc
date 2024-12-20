#!/usr/bin/env bash
# Configuration validation utility functions

# Validate NUMA configuration
validate_numa_config() {
    local xml_file="$1"
    
    # Check NUMA topology consistency
    if ! xmllint --xpath "//numa" "$xml_file" >/dev/null 2>&1; then
        log_error "Missing NUMA configuration in XML"
        return 1
    fi
    
    # Validate memory allocation
    local total_mem
    total_mem=$(xmllint --xpath "sum(//numa/cell/@memory)" "$xml_file")
    local system_mem
    system_mem=$(grep MemTotal /proc/meminfo | awk '{print $2/1024/1024}')
    
    if (( $(echo "$total_mem > $system_mem" | bc -l) )); then
        log_error "Configured memory ($total_mem GiB) exceeds system memory ($system_mem GiB)"
        return 1
    fi
    
    return 0
}

# Validate device configuration
validate_device_config() {
    local xml_file="$1"
    
    # Check device-NUMA relationships
    while IFS= read -r device; do
        local numa_node
        numa_node=$(xmllint --xpath "string($device/@node)" "$xml_file")
        local dev_path
        dev_path=$(xmllint --xpath "string($device/source/@*[local-name()='address'])" "$xml_file")
        
        if [[ -n "$dev_path" ]]; then
            local actual_node
            actual_node=$(get_device_numa_node "$dev_path")
            if [[ "$numa_node" != "$actual_node" && "$actual_node" != "-1" ]]; then
                log_warning "Device $dev_path assigned to non-optimal NUMA node"
            fi
        fi
    done < <(xmllint --xpath "//devices/*[@node]" "$xml_file")
    
    return 0
}

