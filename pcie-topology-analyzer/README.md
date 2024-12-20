# PCIe Topology Analyzer and Libvirt XML Generator

A comprehensive tool for analyzing PCIe topology and generating optimized libvirt XML configurations.

## Features

- Comprehensive hardware topology analysis
- NUMA-aware configuration generation 
- PCIe passthrough optimization
- Detailed logging and debugging
- XML validation
- Custom ignore patterns support
- Token counting capability

## Quick Start

```bash
# Basic analysis
./pcie-topology-analyzer.sh

# With validation and custom output
./pcie-topology-analyzer.sh --validate --output-dir /path/to/output

# Debug mode with token counting
./pcie-topology-analyzer.sh --debug --count-tokens
```

## Requirements

- bash 4.0+
- pciutils
- hwloc
- numactl
- libvirt-clients
- cgpt

## Documentation

See the [User Guide](USER_GUIDE.md) for detailed usage instructions.

## License

MIT License - See LICENSE file for details

