# PCIe Topology Analyzer Test Environment

This directory contains scripts to set up and run a test environment using QEMU/KVM for the PCIe topology analyzer.

## Requirements

- QEMU/KVM
- libvirt
- cloud-image-utils
- Ubuntu cloud image

## Setup

1. Install dependencies:
   ```bash
   sudo apt-get install qemu-kvm qemu-utils cloud-image-utils libvirt-clients libvirt-daemon-system
   ```

2. Run the setup script:
   ```bash
   ./setup-test-vm.sh
   ```

3. Run tests:
   ```bash
   ./run-tests.sh
   ```

## Test Environment Details

The test environment provides:
- Ubuntu 22.04 VM with 8GB RAM
- 2 NUMA nodes
- Virtual PCIe devices
- Pre-installed required packages

## Customization

Edit `setup-test-vm.sh` to modify:
- VM specifications
- PCIe device configuration
- NUMA topology
- Network settings

## Troubleshooting

1. VM not starting:
   ```bash
   virsh start pcie-topology-test
   virsh console pcie-topology-test
   ```

2. Network issues:
   ```bash
   virsh domifaddr pcie-topology-test
   virsh domiflist pcie-topology-test
   ```

3. Check VM status:
   ```bash
   virsh list --all
   virsh dumpxml pcie-topology-test
   ```

