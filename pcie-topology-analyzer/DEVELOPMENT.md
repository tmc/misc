# Development Guide

## Architecture

The analyzer follows a modular design with these components:

### 1. Data Collection Layer

- PCIe topology analysis
  - Device enumeration
  - Relationship mapping
  - IOMMU grouping

- NUMA configuration
  - Memory topology
  - CPU affinity
  - Node relationships

- Hardware layout
  - Device placement
  - Bus hierarchy
  - Resource allocation

### 2. Analysis Engine

- Topology mapping
  - Device relationships
  - Bus connectivity
  - Resource dependencies

- Performance optimization
  - NUMA awareness
  - Memory access patterns
  - CPU pinning strategies

- Resource allocation
  - Memory distribution
  - CPU assignment
  - Device passthrough

### 3. Output Generation

- XML configuration
  - Domain definition
  - Device configuration
  - Resource assignments

- Validation
  - Schema compliance
  - Resource conflicts
  - Configuration integrity

## Development Setup

1. Clone repository:
   ```bash
   git clone https://github.com/tmc/misc
   cd misc/pcie-topology-analyzer
   ```

2. Install development tools:
   ```bash
   # Development dependencies
   sudo apt-get install shellcheck bats
   ```

3. Configure test environment:
   ```bash
   ./setup_dev_env.sh
   ```

## Testing

### Unit Tests

```bash
# Run all tests
make test

# Run specific test suite
make test-topology
make test-numa
make test-xml
```

### Integration Tests

```bash
# Full system test
./integration_tests.sh

# Component tests
./test_pcie_analysis.sh
./test_numa_config.sh
```

### Performance Testing

```bash
# Benchmark script
./benchmark.sh

# Profile execution
./profile.sh
```

## Coding Standards

### Shell Script Guidelines

1. Use shellcheck
2. Follow Google Shell Style Guide
3. Document all functions
4. Include error handling
5. Add tests for new features

### Documentation

1. Update README.md
2. Add function documentation
3. Include usage examples
4. Document configuration options

### Git Workflow

1. Create feature branch
2. Write tests
3. Implement feature
4. Submit pull request

## Contributing

### Pull Request Process

1. Fork repository
2. Create feature branch
3. Add tests
4. Implement changes
5. Submit PR

### Code Review

- Follow review checklist
- Address feedback
- Update documentation
- Maintain test coverage

## Release Process

1. Version bump
2. Update changelog
3. Run test suite
4. Create release tag
5. Update documentation

