#!/bin/bash
# Set up mock /sys filesystem structure

# Create basic directory structure
mkdir -p /sys/devices/pci0000:00
mkdir -p /sys/bus/pci/devices

# Create mock PCI devices
create_pci_device() {
    local device_id="$1"
    local numa_node="$2"
    local sriov="${3:-0}"
    
    local dev_path="/sys/bus/pci/devices/$device_id"
    mkdir -p "$dev_path"
    
    # Create NUMA node file
    echo "$numa_node" > "$dev_path/numa_node"
    
    # Create SR-IOV files if needed
    if [[ "$sriov" -gt 0 ]]; then
        echo "$sriov" > "$dev_path/sriov_totalvfs"
        echo "0" > "$dev_path/sriov_numvfs"
    fi
    
    # Create IOMMU group symlink
    mkdir -p "/sys/kernel/iommu_groups/$numa_node"
    ln -s "/sys/kernel/iommu_groups/$numa_node" "$dev_path/iommu_group"
}

# Create mock devices based on JSON configuration
python3 -c '
import json
with open("/app/test/mock_data/pci_devices.json") as f:
    devices = json.load(f)
for addr, dev in devices.items():
    numa_node = dev.get("numa_node", 0)
    sriov = 8 if "SR-IOV" in dev.get("capabilities", []) else 0
    print(f"{addr} {numa_node} {sriov}")
' | while read -r dev_id numa_node sriov; do
    create_pci_device "$dev_id" "$numa_node" "$sriov"
done

