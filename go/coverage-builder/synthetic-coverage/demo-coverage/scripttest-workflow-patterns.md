# ScriptTest Workflow Patterns for Effective Coverage

When adopting `rsc.io/script/scripttest` in an existing codebase, it's important to identify which workflow patterns will provide the most valuable coverage. This guide outlines common patterns that work well with scripttest and explains how to implement them for maximum coverage effectiveness.

## 1. Command Chain Pattern

The Command Chain pattern tests a sequence of commands that build upon each other. This is perfect for scripttest since it naturally models real user workflows.

### Example

```
# user-workflow.txt
# User creation and management workflow

# Setup - create a user
exec mytool user create --name=testuser --email=user@example.com
stdout 'User created successfully'
! stderr .

# Grant admin privileges
exec mytool user grant-role testuser admin
stdout 'Role granted successfully'
! stderr .

# Verify user info shows admin role
exec mytool user info testuser
stdout 'Name: testuser'
stdout 'Email: user@example.com'
stdout 'Roles: admin'
! stderr .

# Revoke admin privileges
exec mytool user revoke-role testuser admin
stdout 'Role revoked successfully'
! stderr .

# Verify role was removed
exec mytool user info testuser
stdout 'Name: testuser'
stdout 'Email: user@example.com'
stdout 'Roles:'
! stdout 'Roles: admin'
! stderr .

# Delete the user
exec mytool user delete testuser --confirm
stdout 'User deleted successfully'
! stderr .

# Verify user no longer exists
! exec mytool user info testuser
stderr 'User not found'
```

### Coverage Benefits

This pattern exercises:
- Command chaining and state persistence
- Full CRUD operations
- Permission management
- Error handling for nonexistent resources

### Code Mapping

```yaml
# command-map.yaml
commands:
  - name: "user create"
    functions:
      - file: "internal/user/create.go"
        function: "CreateUser"
        start_line: 15
        end_line: 45
  - name: "user grant-role"
    functions:
      - file: "internal/user/roles.go"
        function: "GrantRole"
        start_line: 10
        end_line: 30
  # ... additional mappings
```

## 2. State Validation Pattern

This pattern focuses on creating, modifying, and validating application state. It's particularly useful for configuration management and data storage operations.

### Example

```
# config-validation.txt
# Configuration state management and validation

# Create initial config
exec mytool config init
stdout 'Config initialized'
! stderr .

# Set various config values
exec mytool config set database.host localhost
exec mytool config set database.port 5432
exec mytool config set database.user postgres
exec mytool config set logging.level debug

# Validate specific settings
exec mytool config get database.host
stdout 'localhost'

exec mytool config get database.port
stdout '5432'

# Export and verify the config file
exec mytool config export --format=json
stdout '{"database":{"host":"localhost","port":5432,"user":"postgres"},"logging":{"level":"debug"}}'
! stderr .

# Test validation rules
! exec mytool config set database.port invalid-port
stderr 'Invalid port: must be a number'

! exec mytool config set logging.level extreme
stderr 'Invalid log level'

# Reset config and verify it's empty
exec mytool config reset --confirm
stdout 'Config reset'

exec mytool config export --format=json
stdout '{}'
! stderr .
```

### Coverage Benefits

This pattern exercises:
- Configuration parsing and validation
- JSON/YAML handling
- Type checking and validation rules
- Default value management
- Configuration persistence

### Code Mapping

```yaml
commands:
  - name: "config init"
    functions:
      - file: "internal/config/init.go"
        function: "InitConfig"
        start_line: 10
        end_line: 25
  - name: "config set"
    functions:
      - file: "internal/config/set.go"
        function: "SetConfigValue"
        start_line: 15
        end_line: 60
      - file: "internal/config/validation.go"
        function: "ValidateConfigValue"
        start_line: 5
        end_line: 45
  # ... additional mappings
```

## 3. Error Handling Pattern

The Error Handling pattern systematically tests various error conditions and edge cases to ensure your application handles them properly.

### Example

```
# error-handling.txt
# Comprehensive error handling test

# Test with nonexistent file
! exec mytool process file-doesnt-exist.txt
stderr 'Error: file not found'

# Test with empty file
>empty.txt

! exec mytool process empty.txt
stderr 'Error: file is empty'

# Test with invalid JSON
>invalid.json invalid json content

! exec mytool process --format=json invalid.json
stderr 'Error: invalid JSON'

# Test with read-only file
>readonly.txt some content
chmod 0400 readonly.txt

! exec mytool modify readonly.txt
stderr 'Error: permission denied'

# Test with full disk simulation
! exec mytool export --simulate-full-disk
stderr 'Error: no space left on device'

# Test with network timeout
! exec mytool fetch --simulate-timeout
stderr 'Error: network timeout'

# Test with invalid credentials
! exec mytool login --user=invalid --password=wrong
stderr 'Error: invalid credentials'

# Test with rate limiting
! exec mytool api-call --simulate-rate-limit
stderr 'Error: rate limit exceeded'
```

### Coverage Benefits

This pattern exercises:
- Error handling code paths
- Validation logic
- Filesystem error handling
- Network error handling
- Authentication failures
- System resource limitations

### Code Mapping

```yaml
commands:
  - name: "process"
    functions:
      - file: "internal/processor/file.go"
        function: "ProcessFile"
        start_line: 25
        end_line: 80
      - file: "internal/processor/errors.go"
        function: "HandleFileError"
        start_line: 10
        end_line: 50
  # ... additional mappings
```

## 4. Resource Management Pattern

This pattern tests the lifecycle of resources, ensuring they're properly created, used, and cleaned up.

### Example

```
# resource-lifecycle.txt
# Resource lifecycle management test

# Create a temporary workspace
exec mytool workspace create --name=temp
stdout 'Workspace created: temp'
! stderr .

# Add resources to the workspace
exec mytool workspace add temp --resource=file1.txt
stdout 'Resource added'
exec mytool workspace add temp --resource=file2.txt
stdout 'Resource added'

# List resources
exec mytool workspace list temp
stdout 'file1.txt'
stdout 'file2.txt'

# Use the workspace
exec mytool workspace run temp --command=process
stdout 'Processing 2 resources'
stdout 'Completed successfully'

# Remove a resource
exec mytool workspace remove temp --resource=file1.txt
stdout 'Resource removed'

# Verify resource was removed
exec mytool workspace list temp
stdout 'file2.txt'
! stdout 'file1.txt'

# Clean up the workspace
exec mytool workspace delete temp --confirm
stdout 'Workspace deleted'

# Verify workspace is gone
! exec mytool workspace list temp
stderr 'Workspace not found'
```

### Coverage Benefits

This pattern exercises:
- Resource allocation and deallocation
- Reference counting
- Cleanup operations
- Resource locking
- Dependency management

### Code Mapping

```yaml
commands:
  - name: "workspace create"
    functions:
      - file: "internal/workspace/create.go"
        function: "CreateWorkspace"
        start_line: 20
        end_line: 55
  - name: "workspace add"
    functions:
      - file: "internal/workspace/resources.go"
        function: "AddResource"
        start_line: 30
        end_line: 65
  # ... additional mappings
```

## 5. Transformation Pipeline Pattern

This pattern tests data transformation pipelines, where data flows through multiple processing steps.

### Example

```
# data-pipeline.txt
# Data transformation pipeline test

# Create test data
>input.csv
id,name,value
1,item1,100
2,item2,200
3,item3,300

# Step 1: Load the data
exec mytool data load --file=input.csv --format=csv
stdout 'Loaded 3 records'
! stderr .

# Step 2: Transform the data
exec mytool data transform --operation=multiply-value --factor=2
stdout 'Transformed 3 records'
! stderr .

# Step 3: Filter the data
exec mytool data filter --condition="value > 300"
stdout 'Filtered to 2 records'
! stderr .

# Step 4: Sort the data
exec mytool data sort --by=value --order=desc
stdout 'Sorted 2 records'
! stderr .

# Step 5: Export the result
exec mytool data export --output=result.json --format=json
stdout 'Exported 2 records'
! stderr .

# Verify the result
exec cat result.json
stdout '{"records":\['
stdout '{"id":3,"name":"item3","value":600}'
stdout '{"id":2,"name":"item2","value":400}'
stdout '\]}'
```

### Coverage Benefits

This pattern exercises:
- Data parsing and serialization
- Transformation logic
- Filtering algorithms
- Sorting functions
- Export formatting

### Code Mapping

```yaml
commands:
  - name: "data load"
    functions:
      - file: "internal/data/loader.go"
        function: "LoadFromFile"
        start_line: 15
        end_line: 75
      - file: "internal/data/formats/csv.go"
        function: "ParseCSV"
        start_line: 10
        end_line: 60
  # ... additional mappings
```

## 6. Concurrency Pattern

This pattern tests concurrent operations and ensures they work correctly without race conditions or deadlocks.

### Example

```
# concurrency.txt
# Concurrent operations test

# Set up a test server that handles multiple concurrent requests
exec mytool server start --port=8080 --background
stdout 'Server started'
! stderr .

# Send multiple concurrent requests
exec mytool client request --concurrency=10 --requests=100 --target=http://localhost:8080
stdout 'Completed 100 requests'
stdout 'Success rate: 100%'
stdout 'Average response time:'
! stderr .

# Check server stats
exec mytool server stats
stdout 'Total requests: 100'
stdout 'Concurrent connections peak:'
stdout 'No errors reported'
! stderr .

# Test rate limiting
exec mytool client request --concurrency=20 --requests=100 --target=http://localhost:8080 --rate-limit=10
stdout 'Completed 100 requests'
stdout 'Rate limited: true'
! stderr .

# Shutdown the server
exec mytool server stop
stdout 'Server stopped'
! stderr .
```

### Coverage Benefits

This pattern exercises:
- Goroutine management
- Channel operations
- Mutex and lock usage
- Context handling
- Graceful shutdown
- Rate limiting

### Code Mapping

```yaml
commands:
  - name: "server start"
    functions:
      - file: "internal/server/server.go"
        function: "StartServer"
        start_line: 25
        end_line: 75
      - file: "internal/server/handlers.go"
        function: "SetupHandlers"
        start_line: 15
        end_line: 50
  # ... additional mappings
```

## 7. Integration Pattern

This pattern tests integration with external systems or components.

### Example

```
# integration.txt
# External system integration test

# Set up a mock server for integration testing
exec mytool mock-server start --service=database --port=5000
stdout 'Mock database server started on port 5000'
! stderr .

# Configure the tool to use the mock server
exec mytool config set database.url localhost:5000
stdout 'Configuration updated'

# Test database operations
exec mytool db create-table --name=users
stdout 'Table created'
! stderr .

exec mytool db insert --table=users --data='{"id":1,"name":"test"}'
stdout 'Record inserted'
! stderr .

exec mytool db query --table=users --where='id=1'
stdout '{"id":1,"name":"test"}'
! stderr .

# Test error handling with the mock
exec mytool mock-server configure --service=database --error=connection-drop
stdout 'Mock configured to simulate connection drop'

! exec mytool db query --table=users --where='id=1'
stderr 'Database connection error'

# Shut down mock server
exec mytool mock-server stop --service=database
stdout 'Mock database server stopped'
! stderr .
```

### Coverage Benefits

This pattern exercises:
- External API client code
- Connection management
- Request/response handling
- Error recovery
- Retry logic
- Timeout handling

### Code Mapping

```yaml
commands:
  - name: "mock-server start"
    functions:
      - file: "internal/testing/mocks/server.go"
        function: "StartMockServer"
        start_line: 20
        end_line: 60
  - name: "db create-table"
    functions:
      - file: "internal/db/schema.go"
        function: "CreateTable"
        start_line: 25
        end_line: 55
      - file: "internal/db/client.go"
        function: "ExecuteDDL"
        start_line: 100
        end_line: 135
  # ... additional mappings
```

## 8. Upgrade/Migration Pattern

This pattern tests version upgrades, data migrations, and backward compatibility.

### Example

```
# migration.txt
# Database migration and upgrade test

# Create old format data
>old-data.json
{"version":"1.0","items":[{"id":1,"legacy_name":"item1"},{"id":2,"legacy_name":"item2"}]}

# Import old format
exec mytool import --file=old-data.json
stdout 'Imported 2 items'
stdout 'Detected legacy format (1.0)'
! stderr .

# Run migration
exec mytool migrate --from=1.0 --to=2.0
stdout 'Migration started'
stdout 'Converting legacy_name to name'
stdout 'Migration completed'
! stderr .

# Verify migrated data
exec mytool export --format=json
stdout '{"version":"2.0","items":\[{"id":1,"name":"item1"},{"id":2,"name":"item2"}\]}'
! stdout 'legacy_name'
! stderr .

# Test backward compatibility
exec mytool export --format=json --compat-version=1.0
stdout '{"version":"1.0","items":\[{"id":1,"legacy_name":"item1"},{"id":2,"legacy_name":"item2"}\]}'
! stderr .
```

### Coverage Benefits

This pattern exercises:
- Schema migration code
- Version detection logic
- Data transformation for compatibility
- Backward compatibility logic
- Version-specific code paths

### Code Mapping

```yaml
commands:
  - name: "import"
    functions:
      - file: "internal/import/importer.go"
        function: "ImportFile"
        start_line: 25
        end_line: 80
      - file: "internal/versions/detector.go"
        function: "DetectVersion"
        start_line: 15
        end_line: 45
  - name: "migrate"
    functions:
      - file: "internal/migrations/engine.go"
        function: "RunMigration"
        start_line: 30
        end_line: 100
      - file: "internal/migrations/transforms.go"
        function: "TransformData"
        start_line: 20
        end_line: 150
  # ... additional mappings
```

## 9. Authentication & Authorization Pattern

This pattern tests user authentication and authorization flows.

### Example

```
# auth.txt
# Authentication and authorization test

# Register a new user
exec mytool auth register --username=testuser --password=password123 --email=test@example.com
stdout 'User registered successfully'
! stderr .

# Failed login (wrong password)
! exec mytool auth login --username=testuser --password=wrongpassword
stderr 'Invalid credentials'

# Successful login
exec mytool auth login --username=testuser --password=password123
stdout 'Login successful'
stdout 'Token:'
! stderr .
# Save the token
save stdout token

# Use the token for an authorized request
exec mytool resources list --token=$token
stdout 'Resources:'
! stderr .

# Test authorization levels
! exec mytool admin list-users --token=$token
stderr 'Unauthorized: requires admin privileges'

# Grant admin role
exec mytool auth grant-role --username=testuser --role=admin --super-token=admin-token
stdout 'Role granted successfully'
! stderr .

# Now admin request should work
exec mytool admin list-users --token=$token
stdout 'Users:'
! stderr .

# Test token expiration
exec mytool auth invalidate-token --token=$token
stdout 'Token invalidated'

# Verify token no longer works
! exec mytool resources list --token=$token
stderr 'Unauthorized: invalid or expired token'
```

### Coverage Benefits

This pattern exercises:
- User authentication logic
- Password hashing and verification
- Token generation and validation
- Permission checking
- Role-based access control
- Session management

### Code Mapping

```yaml
commands:
  - name: "auth register"
    functions:
      - file: "internal/auth/registration.go"
        function: "RegisterUser"
        start_line: 20
        end_line: 70
      - file: "internal/auth/password.go"
        function: "HashPassword"
        start_line: 15
        end_line: 40
  # ... additional mappings
```

## 10. Plugin/Extension Pattern

This pattern tests plugin or extension loading and execution.

### Example

```
# plugins.txt
# Plugin system test

# List available plugins (none yet)
exec mytool plugin list
stdout 'No plugins installed'
! stderr .

# Install a plugin
exec mytool plugin install --source=./testdata/sample-plugin.zip
stdout 'Plugin "sample" installed successfully'
! stderr .

# List plugins after installation
exec mytool plugin list
stdout 'Installed plugins:'
stdout '- sample (v1.0.0)'
! stderr .

# Get plugin info
exec mytool plugin info sample
stdout 'Name: sample'
stdout 'Version: 1.0.0'
stdout 'Author: Test Author'
stdout 'Commands: sample-command'
! stderr .

# Run plugin command
exec mytool sample-command --arg=test
stdout 'Sample plugin executed with arg: test'
! stderr .

# Update plugin
exec mytool plugin update sample --source=./testdata/sample-plugin-v2.zip
stdout 'Plugin "sample" updated to v2.0.0'
! stderr .

# Check updated version
exec mytool plugin info sample
stdout 'Version: 2.0.0'
! stderr .

# Uninstall plugin
exec mytool plugin uninstall sample
stdout 'Plugin "sample" uninstalled successfully'
! stderr .

# Verify plugin is gone
exec mytool plugin list
stdout 'No plugins installed'
! stderr .
```

### Coverage Benefits

This pattern exercises:
- Plugin discovery and loading
- Dynamic code execution
- Version compatibility checking
- Plugin lifecycle management
- Plugin API and extension points

### Code Mapping

```yaml
commands:
  - name: "plugin install"
    functions:
      - file: "internal/plugins/manager.go"
        function: "InstallPlugin"
        start_line: 45
        end_line: 120
      - file: "internal/plugins/validator.go"
        function: "ValidatePlugin"
        start_line: 20
        end_line: 75
  # ... additional mappings
```

## Implementing These Patterns Effectively

To get the most coverage benefit from these patterns:

### 1. Start with High-Level Workflows

Begin by implementing the Command Chain pattern for your most important user workflows. This provides a foundation of integration testing that exercises multiple components.

### 2. Add Specialized Pattern Tests

Once basic workflows are covered, add specialized tests for each applicable pattern. Focus on areas with complex logic or error-prone code.

### 3. Create Test Data Helpers

ScriptTest allows creating test files with the `>filename` syntax. Create helper functions to generate complex test data:

```go
// test/scripttest/helpers/data_generator.go
package helpers

import (
	"encoding/json"
	"fmt"
	"os"
)

// GenerateTestData creates test data files for scripttest
func GenerateTestData(dir string) error {
	// Create test users JSON
	users := []map[string]interface{}{
		{"id": 1, "name": "User 1", "email": "user1@example.com"},
		{"id": 2, "name": "User 2", "email": "user2@example.com"},
	}
	
	usersJSON, _ := json.MarshalIndent(users, "", "  ")
	if err := os.WriteFile(dir+"/users.json", usersJSON, 0644); err != nil {
		return err
	}
	
	// Create other test files as needed
	// ...
	
	return nil
}
```

### 4. Maintain Command Mappings

As your codebase evolves, keep your command mappings up to date. Consider automating this process using AST analysis:

```go
// tools/update-command-map/main.go
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"gopkg.in/yaml.v2"
)

// Load existing mappings, analyze codebase, update mappings
// ...
```

### 5. Use Template Scripts

For similar commands, use templates to generate test scripts:

```go
// tools/generate-scripts/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"text/template"
)

var crudTemplate = `# {{.Resource}} CRUD operations

# Create
exec mytool {{.Resource}} create --name=test{{.Resource}}
stdout '{{.Resource | title}} created'
! stderr .

# Read
exec mytool {{.Resource}} get test{{.Resource}}
stdout 'Name: test{{.Resource}}'
! stderr .

# Update
exec mytool {{.Resource}} update test{{.Resource}} --field=value
stdout '{{.Resource | title}} updated'
! stderr .

# Delete
exec mytool {{.Resource}} delete test{{.Resource}}
stdout '{{.Resource | title}} deleted'
! stderr .

# Verify deleted
! exec mytool {{.Resource}} get test{{.Resource}}
stderr '{{.Resource | title}} not found'
`

// Generate CRUD scripts for different resources
// ...
```

## Coverage Analysis and Reporting

To get the most value from scripttest coverage:

### 1. Compare Coverage Before and After

```go
// tools/coverage-diff/main.go
package main

import (
	"flag"
	"fmt"
	"os"
)

// Compare coverage before and after adding scripttest
// ...
```

### 2. Identify Coverage Gaps

```go
// tools/coverage-gaps/main.go
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// Find functions with low coverage
// ...
```

### 3. Track Coverage Over Time

```go
// Setup a coverage database
// Record coverage metrics over time
// Generate trend reports
// ...
```

## Conclusion

By implementing these patterns in your scripttest files, you can achieve comprehensive coverage of your codebase while testing real-world usage scenarios. The key benefits are:

1. **Complete Workflow Testing**: Tests how users actually use your application
2. **Edge Case Coverage**: Systematically tests error conditions and edge cases
3. **Integration Verification**: Ensures components work together correctly
4. **Realistic Usage Patterns**: Tests reflect actual usage, not just unit-level functionality
5. **Documentation**: Scripts serve as executable documentation of expected behavior

Remember that scripttest is most effective when combined with traditional unit tests. Unit tests provide fast feedback and detailed testing of edge cases, while scripttest tests provide end-to-end validation of complete workflows.

To maximize the value of synthetic coverage, focus on implementing the patterns that best match your application's critical functionality and ensure your command mappings accurately reflect the code paths that each command exercises.