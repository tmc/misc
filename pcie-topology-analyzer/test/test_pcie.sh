#!/usr/bin/env bash
# PCIe functionality tests
source "$(dirname "${BASH_SOURCE[0]}")/../lib/pcie_utils.sh"

test_pcie_detection() {
    # Test PCIe device detection
    local devices
    devices=$(lspci -D)
    
    if [[ -z "$devices" ]]; then
        echo "FAIL: Could not detect PCIe devices"
        return 1
    fi
    
    # Test IOMMU group detection
    while IFS= read -r dev; do
        local dev_id
        dev_id=$(echo "$dev" | awk '{print $1}')
        local iommu_group
        iommu_group=$(get_iommu_group "$dev_id")
        
        if [[ "$iommu_group" == "none" ]]; then
            echo "WARNING: No IOMMU group for device $dev_id"
        fi
    done <<< "$devices"
    
    echo "PASS: PCIe detection tests"
    return 0
}

test_sriov_detection() {
    # Test SR-IOV capability detection
    local sriov_capable=0
    
    while IFS= read -r dev; do
        local dev_id
        dev_id=$(echo "$dev" | awk '{print $1}')
        if check_sriov_support "$dev_id"; then
            sriov_capable=$((sriov_capable + 1))
            
            # Test VF count retrieval
            local max_vfs
            max_vfs=$(get_max_vfs "$dev_id")
            if [[ -z "$max_vfs" ]]; then
                echo "FAIL: Could not get max VFs for SR-IOV device $dev_id"
                return 1
            fi
        fi
    done <<< "$(lspci -D)"
    
    echo "PASS: SR-IOV detection tests (found $sriov_capable capable devices)"
    return 0
}

