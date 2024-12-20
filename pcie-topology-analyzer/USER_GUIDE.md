# PCIe Topology Analyzer User Guide

## Table of Contents

1. [Installation](#installation)
2. [Basic Usage](#basic-usage)
3. [Advanced Usage](#advanced-usage)
4. [Configuration](#configuration)
5. [Examples](#examples)
6. [Troubleshooting](#troubleshooting)

## Installation

### Prerequisites

```bash
# Debian/Ubuntu
sudo apt-get install pciutils hwloc numactl libvirt-clients

# RHEL/CentOS
sudo dnf install pciutils hwloc numactl libvirt-client
```

### Installation Steps

1. Clone the repository:
   ```bash
   git clone https://github.com/tmc/misc
   cd misc/pcie-topology-analyzer
   ```

2. Install dependencies:
   ```bash
   ./install.sh
   ```

3. Verify installation:
   ```bash
   pcie-topology-analyzer --help
   ```

## Basic Usage

### Simple Analysis

```bash
pcie-topology-analyzer
```

This will:
- Analyze PCIe topology
- Collect NUMA information
- Generate basic XML configuration

### With Validation

```bash
pcie-topology-analyzer --validate
```

### Custom Output Directory

```bash
pcie-topology-analyzer --output-dir /path/to/output
```

## Advanced Usage

### Debug Mode

Enable detailed logging:
```bash
pcie-topology-analyzer --debug
```

### NUMA Optimization

For NUMA-aware configurations:
```bash
pcie-topology-analyzer --numa-optimize
```

### Custom Configurations

1. Create `.pcie-topology-ignore` file
2. Add custom exclude patterns
3. Run analyzer

## Configuration

### Environment Variables

- `PCIE_ANALYZER_DEBUG`: Enable debug mode
- `PCIE_ANALYZER_OUTPUT`: Set output directory
- `PCIE_ANALYZER_VALIDATE`: Enable validation

### Configuration Files

1. `.pcie-topology-ignore`: Custom ignore patterns
2. `config.yaml`: Advanced configuration options

## Examples

### Basic GPU Passthrough

```bash
pcie-topology-analyzer --template gpu-passthrough
```

### Complex NUMA Configuration

```bash
pcie-topology-analyzer --numa-aware --cpu-pinning
```

## Troubleshooting

### Common Issues

1. Permission denied:
   ```bash
   sudo pcie-topology-analyzer
   ```

2. Missing tools:
   ```bash
   # Install required tools
   sudo apt-get install pciutils hwloc numactl
   ```

3. Validation errors:
   - Check XML syntax
   - Verify IOMMU groups
   - Check NUMA configuration

### Debug Output

Enable debug logging:
```bash
pcie-topology-analyzer --debug 2>debug.log
```

### Support

For additional support:
1. Check documentation
2. Submit GitHub issue
3. Contact maintainers

