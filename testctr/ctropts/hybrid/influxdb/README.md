# Influxdb Module

Generated hybrid module for influxdb containers. This module provides a starting point that combines generated boilerplate with manual enhancement areas.

## Status: Ready for Manual Enhancement

### What's Generated ✅
- Basic module structure
- Default configuration
- 4 discovered configuration options
- DSN provider scaffolding
- Standard testctr integration

### What Needs Manual Work 🔨

#### DSN Provider Implementation
- [ ] Complete CreateDatabase() implementation
- [ ] Complete DropDatabase() implementation  
- [ ] Enhance FormatDSN() with proper parameters
- [ ] Add connection pooling/retry logic


#### Service-Specific Features
- [ ] Add configuration file mounting
- [ ] Add initialization script support
- [ ] Add clustering/replication support
- [ ] Add security/authentication helpers
- [ ] Add health check functions
- [ ] Add backup/restore utilities

#### Testing
- [ ] Add comprehensive unit tests
- [ ] Add integration tests with real influxdb operations
- [ ] Add performance/load tests
- [ ] Add error handling tests

### Enhancement Guidelines

1. **Follow existing patterns**: Look at mysql2/postgres2 for DSN provider examples
2. **Add comprehensive options**: Study influxdb documentation for all configuration options
3. **Include helpers**: Add convenience functions for common operations
4. **Test thoroughly**: Ensure all functionality works with real influxdb containers
5. **Document well**: Add examples and usage patterns

### Quick Start

```go
// Basic usage
container := testctr.New(t, "influxdb:2", influxdb.Default())

// With DSN support
dsn := container.DSN(t)

// With custom options
container := testctr.New(t, "influxdb:2", 
    influxdb.Default(),
    influxdb.WithUsername("custom-value"),
)
```

### Related Modules
- mysql2/ - Full-featured MySQL implementation  
- postgres2/ - Full-featured PostgreSQL implementation
- gen/influxdb/ - Basic generated version
