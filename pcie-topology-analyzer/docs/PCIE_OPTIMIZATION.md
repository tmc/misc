# PCIe Device Optimization Guide

## Overview

This guide covers PCIe device optimization techniques implemented in the PCIe Topology Analyzer tool.

## Device Analysis

### Topology Discovery

The tool analyzes:
- PCIe device hierarchy
- IOMMU groups
- Device-to-NUMA relationships
- Bandwidth capabilities

### NUMA Locality

PCIe devices have natural NUMA affinity:
- Devices connect to specific NUMA nodes
- Access from other nodes incurs latency
- Proper assignment improves performance

## Optimization Techniques

### Multi-Queue Support

1. virtio-net
```xml
<interface type='network'>
  <model type='virtio'/>
  <driver name='vhost' queues='4'/>
</interface>
```

2. virtio-scsi
```xml
<controller type='scsi' model='virtio-scsi'>
  <driver queues='4'/>
</controller>
```

### Bridge Zero Copy Transmit

Enables zero-copy mode for network bridges:
```bash
echo 1 > /sys/module/vhost_net/parameters/experimental_zcopytx
```

### Device Assignment

1. Direct Device Assignment
```xml
<hostdev mode='subsystem' type='pci' managed='yes'>
  <source>
    <address domain='0x0000' bus='0x01' slot='0x00' function='0x0'/>
  </source>
</hostdev>
```

2. SR-IOV Configuration
```xml
<interface type='hostdev'>
  <source>
    <address domain='0x0000' bus='0x00' slot='0x07' function='0x0'/>
  </source>
</interface>
```

## Performance Considerations

### Interrupt Handling

- MSI/MSI-X support
- Interrupt routing
- CPU affinity

### DMA Operations

- IOMMU grouping
- DMA mapping
- Memory barriers

### Cache Effects

- Cache line alignment
- Cache coherency
- Write combining

## Monitoring and Tuning

### Tools

1. Performance Monitoring
   - `perf`
   - `ethtool`
   - `lspci`

2. Configuration Validation
   - `virsh nodedev-list`
   - `virsh nodedev-dumpxml`

### Optimization Checklist

- [ ] Verify NUMA locality
- [ ] Enable multi-queue support
- [ ] Configure interrupt affinity
- [ ] Validate IOMMU groups
- [ ] Test DMA performance

