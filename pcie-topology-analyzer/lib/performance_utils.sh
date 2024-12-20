#!/usr/bin/env bash
# Performance optimization utility functions

# Configure huge pages
configure_huge_pages() {
    local pages_2mb="$1"
    local pages_1gb="$2"
    
    # Configure 2MB huge pages
    if [[ -n "$pages_2mb" ]]; then
        echo "$pages_2mb" > /proc/sys/vm/nr_hugepages
    fi
    
    # Configure 1GB huge pages (if supported)
    if [[ -n "$pages_1gb" ]] && grep -q "pdpe1gb" /proc/cpuinfo; then
        echo "$pages_1gb" > /sys/kernel/mm/hugepages/hugepages-1048576kB/nr_hugepages
    fi
}

# Configure KSM (Kernel Same-page Merging)
configure_ksm() {
    local enabled="$1"
    local merge_across_nodes="$2"
    
    if [[ "$enabled" == "true" ]]; then
        echo 1 > /sys/kernel/mm/ksm/run
        echo "$merge_across_nodes" > /sys/kernel/mm/ksm/merge_across_nodes
    else
        echo 0 > /sys/kernel/mm/ksm/run
    fi
}

# Configure CPU governor
configure_cpu_governor() {
    local governor="$1"
    
    for cpu in /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor; do
        echo "$governor" > "$cpu"
    done
}

# Enable/disable CPU C-states
configure_cpu_cstates() {
    local enabled="$1"
    
    if [[ "$enabled" == "false" ]]; then
        echo 1 > /sys/devices/system/cpu/cpuidle/state*/disable
    else
        echo 0 > /sys/devices/system/cpu/cpuidle/state*/disable
    fi
}

