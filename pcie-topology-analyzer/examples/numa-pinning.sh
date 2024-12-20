#!/usr/bin/env bash
# Example script demonstrating NUMA pinning configuration

# Get NUMA topology
numa_nodes=$(numactl --hardware | grep 'available:' | awk '{print $2}')

# Generate XML configuration
cat << EOF > numa-config.xml
<?xml version="1.0" encoding="UTF-8"?>
<domain type="kvm">
  <name>numa-optimized</name>
  <memory unit="GiB">32</memory>
  <vcpu placement="static">16</vcpu>
  <numatune>
    <memory mode="strict" nodeset="0-${numa_nodes}"/>
EOF

# Add memory nodes
for node in $(seq 0 $((numa_nodes-1))); do
    cat << EOF >> numa-config.xml
    <memnode cellid="${node}" mode="strict" nodeset="${node}"/>
EOF
done

# Close XML
cat << EOF >> numa-config.xml
  </numatune>
</domain>
EOF

