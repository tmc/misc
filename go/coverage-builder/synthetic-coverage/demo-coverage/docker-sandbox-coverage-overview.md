# Docker and Sandboxing with Synthetic Coverage for Go

## Overview

This document explores approaches for generating synthetic coverage for Go code running in isolated environments like Docker containers or sandboxes. These techniques are especially valuable when:

1. Your tests execute code in isolated containers for security or environmental stability
2. You're testing CLI tools with `rsc.io/script/scripttest` where the application runs in a separate process
3. You need to test code that interacts with system resources in a controlled environment
4. You want to ensure accurate coverage reporting despite process boundaries

## Key Challenges

Isolated execution environments introduce several challenges for code coverage:

1. **Process Boundaries**: Standard Go coverage tools can only track coverage within a single process
2. **Filesystem Isolation**: Container filesystems are isolated, making it difficult to extract coverage data
3. **Permission Restrictions**: Sandboxed environments may have limited permissions to write coverage data
4. **Environment Differences**: Differences between test and production environments may affect coverage
5. **Instrumentation Limitations**: Binary instrumentation may not work correctly across container boundaries

## Solution Architecture

The solution combines several techniques:

1. **Volume Mounting**: Mount shared volumes to capture coverage data from containers
2. **Pre-instrumented Binaries**: Use pre-instrumented binaries for testing in containers
3. **Coverage Exporting**: Export coverage data from isolated environments automatically
4. **Synthetic Coverage**: Generate synthetic coverage for container-executed code paths
5. **Merge Strategy**: Merge real coverage with synthetic coverage to produce accurate reports

In the following sections, we'll explore each of these techniques in detail with practical examples.