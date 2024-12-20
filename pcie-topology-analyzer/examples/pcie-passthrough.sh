#!/usr/bin/env bash
# Example script demonstrating PCIe device passthrough

# Get GPU device address
gpu_address=$(lspci -nn | grep VGA | head -1 | cut -d' ' -f1)

# Generate XML configuration
cat << EOF > pcie-passthrough.xml
<?xml version="1.0" encoding="UTF-8"?>
<domain type="kvm">
  <name>gpu-passthrough</name>
  <memory unit="GiB">16</memory>
  <vcpu placement="static">8</vcpu>
  <devices>
    <hostdev mode="subsystem" type="pci" managed="yes">
      <source>
        <address domain="0x0000" bus="0x${gpu_address:0:2}" slot="0x${gpu_address:3:2}" function="0x${gpu_address:6:1}"/>
      </source>
    </hostdev>
  </devices>
</domain>
EOF
