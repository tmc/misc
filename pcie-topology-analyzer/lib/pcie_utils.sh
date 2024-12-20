#!/usr/bin/env bash
# PCIe utility functions

# Get IOMMU group for device
get_iommu_group() {
    local device_id="$1"
    local iommu_group
    
    iommu_group=$(basename "$(readlink -f "/sys/bus/pci/devices/$device_id/iommu_group" 2>/dev/null)" \
                  2>/dev/null || echo "none")
    echo "$iommu_group"
}

# Check if device supports SR-IOV
check_sriov_support() {
    local device_id="$1"
    
    [[ -e "/sys/bus/pci/devices/$device_id/sriov_totalvfs" ]]
}

# Get maximum VF count for SR-IOV device
get_max_vfs() {
    local device_id="$1"
    
    if check_sriov_support "$device_id"; then
        cat "/sys/bus/pci/devices/$device_id/sriov_totalvfs"
    else
        echo "0"
    fi
}

# Check if device supports MSI-X
check_msix_support() {
    local device_id="$1"
    
    lspci -vvv -s "$device_id" | grep -q "MSI-X"
}

# Get device capabilities
get_device_capabilities() {
    local device_id="$1"
    local caps=()
    
    # Check various capabilities
    if check_sriov_support "$device_id"; then
        caps+=("SR-IOV")
    fi
    
    if check_msix_support "$device_id"; then
        caps+=("MSI-X")
    fi
    
    # Check for ACS (Access Control Services)
    if lspci -vvv -s "$device_id" | grep -q "Access Control Services"; then
        caps+=("ACS")
    fi
    
    # Check for PCI Express capabilities
    if lspci -vvv -s "$device_id" | grep -q "Express (v[0-9])"; then
        caps+=("PCIe")
    fi
    
    echo "${caps[*]}"
}

#[Let me know if you'd like me to continue with more implementation details or documentation]
