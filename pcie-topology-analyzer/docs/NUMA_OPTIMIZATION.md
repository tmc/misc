# NUMA Optimization Guide

## Overview

This guide describes NUMA (Non-Uniform Memory Access) optimization strategies implemented in the PCIe Topology Analyzer tool.

## NUMA Concepts

### Node Distance and Memory Access

NUMA systems have varying memory access times depending on which CPU accesses which memory region. The tool analyzes:
- Node distances
- Memory access patterns
- CPU-to-memory locality

### Memory Policies

The tool supports three memory allocation policies:

1. `strict`
   - Memory allocated strictly from specified nodes
   - Fails if memory unavailable
   - Best performance but least flexible

2. `preferred`
   - Attempts to allocate from specified nodes
   - Falls back to other nodes if necessary
   - Good balance of performance and reliability

3. `interleave`
   - Spreads memory across nodes
   - Round-robin allocation
   - Useful for workloads accessing memory randomly

## Optimization Strategies

### Automatic NUMA Balancing

The tool can enable automatic NUMA balancing which:
- Moves tasks closer to their memory
- Migrates memory to active nodes
- Optimizes page placement

Configuration:
```xml
<numatune>
  <memory mode='strict' placement='auto'/>
</numatune>
```

### CPU Pinning

Proper CPU pinning ensures:
- vCPUs stay on assigned physical CPUs
- Cache efficiency is maintained
- Memory access is localized

Example configuration:
```xml
<cputune>
  <vcpupin vcpu='0' cpuset='0'/>
  <vcpupin vcpu='1' cpuset='2'/>
  <emulatorpin cpuset='1,3'/>
</cputune>
```

### Memory Node Assignment

Best practices:
1. Keep VM resources within single NUMA node if possible
2. Align memory and CPU assignments
3. Consider device locality

## Performance Monitoring

Tools for monitoring NUMA performance:
- `numastat`: Memory statistics
- `numad`: Automatic NUMA balancing daemon
- `lstopo`: Hardware topology visualization

## Troubleshooting

Common issues and solutions:

1. Memory allocation failures
   - Check available memory per node
   - Consider using 'preferred' policy
   - Verify memory isn't fragmented

2. Poor performance
   - Verify CPU pinning configuration
   - Check for cross-node memory access
   - Monitor node distances

