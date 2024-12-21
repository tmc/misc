<cli_structure>
# linear-cli - A comprehensive CLI tool for interacting with the Linear GraphQL API

## auth
- Description: Manage authentication and authorization
- Options and flags:
  - --login: Authenticate with Linear
  - --logout: Log out and remove stored credentials
  - --token <token>: Set API token manually
- Example usage: `linear-cli auth --login`
- Code snippet:
  ```go
  func handleAuth(cmd *cobra.Command, args []string) error {
    if loginFlag, _ := cmd.Flags().GetBool("login"); loginFlag {
      return performLogin()
    }
    if logoutFlag, _ := cmd.Flags().GetBool("logout"); logoutFlag {
      return performLogout()
    }
    if token, _ := cmd.Flags().GetString("token"); token != "" {
      return setAPIToken(token)
    }
    return fmt.Errorf("no valid auth action specified")
  }
  ```

## issue
- Description: Manage issues
- Subcommands:
  - create: Create a new issue
  - update: Update an existing issue
  - list: List issues
  - get: Get details of a specific issue
  - delete: Delete an issue
- Options and flags (for create/update):
  - --title: Issue title
  - --description: Issue description
  - --assignee: Assignee ID
  - --status: Issue status
  - --priority: Issue priority
- Example usage: `linear-cli issue create --title "New feature" --description "Implement user profile page"`
- Code snippet:
  ```go
  func createIssue(cmd *cobra.Command, args []string) error {
    title, _ := cmd.Flags().GetString("title")
    description, _ := cmd.Flags().GetString("description")
    // ... get other flags

    input := &IssueCreateInput{
      Title:       title,
      Description: description,
      // ... set other fields
    }

    result, err := client.CreateIssue(input)
    if err != nil {
      return err
    }

    // Format and display the result
    return nil
  }
  ```

## project
- Description: Manage projects
- Subcommands:
  - create: Create a new project
  - update: Update an existing project
  - list: List projects
  - get: Get details of a specific project
  - delete: Delete a project
- Options and flags (for create/update):
  - --name: Project name
  - --description: Project description
  - --team: Team ID
  - --status: Project status
- Example usage: `linear-cli project list --team-id ABC123`

## team
- Description: Manage teams
- Subcommands:
  - create: Create a new team
  - update: Update an existing team
  - list: List teams
  - get: Get details of a specific team
  - delete: Delete a team
- Options and flags (for create/update):
  - --name: Team name
  - --key: Team key
  - --description: Team description
- Example usage: `linear-cli team create --name "Backend Team" --key BACK`

## user
- Description: Manage users
- Subcommands:
  - list: List users
  - get: Get details of a specific user
  - update: Update user information
- Options and flags:
  - --id: User ID
  - --name: User name
  - --email: User email
- Example usage: `linear-cli user get --id USER123`

## comment
- Description: Manage comments
- Subcommands:
  - create: Create a new comment
  - update: Update an existing comment
  - list: List comments for an issue
  - delete: Delete a comment
- Options and flags:
  - --issue-id: ID of the issue to comment on
  - --body: Comment body
- Example usage: `linear-cli comment create --issue-id ISSUE123 --body "Great progress!"`

## workflow
- Description: Manage workflow states
- Subcommands:
  - list: List workflow states
  - create: Create a new workflow state
  - update: Update an existing workflow state
  - delete: Delete a workflow state
- Options and flags:
  - --team-id: Team ID
  - --name: State name
  - --type: State type (e.g., backlog, unstarted, started, completed)
- Example usage: `linear-cli workflow list --team-id TEAM123`

## roadmap
- Description: Manage roadmaps
- Subcommands:
  - create: Create a new roadmap
  - update: Update an existing roadmap
  - list: List roadmaps
  - get: Get details of a specific roadmap
  - delete: Delete a roadmap
- Options and flags:
  - --name: Roadmap name
  - --description: Roadmap description
- Example usage: `linear-cli roadmap create --name "Q3 2023 Plans"`

## integration
- Description: Manage integrations
- Subcommands:
  - list: List available integrations
  - enable: Enable an integration
  - disable: Disable an integration
  - configure: Configure integration settings
- Options and flags:
  - --type: Integration type (e.g., github, slack)
- Example usage: `linear-cli integration enable --type github`

</cli_structure>

<project_structure>
linear-cli/
├── cmd/
│   ├── root.go
│   ├── auth.go
│   ├── issue.go
│   ├── project.go
│   ├── team.go
│   ├── user.go
│   ├── comment.go
│   ├── workflow.go
│   ├── roadmap.go
│   └── integration.go
├── pkg/
│   ├── api/
│   │   ├── client.go
│   │   ├── queries.go
│   │   └── mutations.go
│   ├── auth/
│   │   ├── login.go
│   │   └── token.go
│   ├── output/
│   │   ├── formatter.go
│   │   └── printer.go
│   └── utils/
│       ├── config.go
│       └── helpers.go
├── internal/
│   └── plugins/
│       └── loader.go
├── main.go
├── go.mod
├── go.sum
├── README.md
└── LICENSE
</project_structure>

<code_snippets>
1. Main entry point of the CLI (main.go):
```go
package main

import (
	"fmt"
	"os"

	"github.com/your-username/linear-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
```

2. GraphQL query execution (pkg/api/client.go):
```go
package api

import (
	"context"

	"github.com/machinebox/graphql"
)

type Client struct {
	client *graphql.Client
}

func NewClient(endpoint string) *Client {
	return &Client{
		client: graphql.NewClient(endpoint),
	}
}

func (c *Client) ExecuteQuery(query string, variables map[string]interface{}, result interface{}) error {
	req := graphql.NewRequest(query)

	for key, value := range variables {
		req.Var(key, value)
	}

	return c.client.Run(context.Background(), req, result)
}
```

3. Authentication handling (pkg/auth/token.go):
```go
package auth

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

type TokenStore struct {
	Token string `json:"token"`
}

func SaveToken(token string) error {
	store := TokenStore{Token: token}
	data, err := json.Marshal(store)
	if err != nil {
		return err
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	tokenFile := filepath.Join(configDir, "linear-cli", "token.json")
	return ioutil.WriteFile(tokenFile, data, 0600)
}

func GetToken() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	tokenFile := filepath.Join(configDir, "linear-cli", "token.json")
	data, err := ioutil.ReadFile(tokenFile)
	if err != nil {
		return "", err
	}

	var store TokenStore
	if err := json.Unmarshal(data, &store); err != nil {
		return "", err
	}

	return store.Token, nil
}
```

4. Output formatting (pkg/output/formatter.go):
```go
package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

func FormatJSON(data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonData))
	return nil
}

func FormatTable(headers []string, rows [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetBorders(tablewriter.Border{Left: true, Top: false, Right: true, Bottom: false})
	table.SetCenterSeparator("|")
	table.AppendBulk(rows)
	table.Render()
}
```

5. Error handling (cmd/root.go):
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "linear-cli",
	Short: "A CLI tool for interacting with the Linear API",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.linear-cli.yaml)")
}

func initConfig() {
	// Read config file and set up global configuration
}

func handleError(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}
```

6. Plugin system design (internal/plugins/loader.go):
```go
package plugins

import (
	"fmt"
	"plugin"
)

type Plugin interface {
	Name() string
	Execute(args []string) error
}

func LoadPlugin(path string) (Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	symPlugin, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("plugin does not export 'Plugin' symbol: %w", err)
	}

	plugin, ok := symPlugin.(Plugin)
	if !ok {
		return nil, fmt.Errorf("plugin does not implement Plugin interface")
	}

	return plugin, nil
}
```
</code_snippets>

<documentation>
# Linear CLI Documentation

## Installation

To install the Linear CLI, follow these steps:

1. Ensure you have Go 1.16 or later installed on your system.
2. Run the following command to install the CLI:

```
go install github.com/your-username/linear-cli@latest
```

3. Verify the installation by running:

```
linear-cli --version
```

## Usage Guide

The Linear CLI follows a command-subcommand structure. The general syntax is:

```
linear-cli <command> <subcommand> [options and flags]
```

To get help for any command or subcommand, use the `--help` flag:

```
linear-cli --help
linear-cli issue --help
```

### Authentication

Before using the CLI, you need to authenticate with Linear:

```
linear-cli auth --login
```

Follow the prompts to complete the authentication process.

## Command Reference

### Issues

- Create an issue: `linear-cli issue create --title "New feature" --description "Implement user profile page"`
- List issues: `linear-cli issue list`
- Get issue details: `linear-cli issue get --id ISSUE123`
- Update an issue: `linear-cli issue update --id ISSUE123 --status "In Progress"`
- Delete an issue: `linear-cli issue delete --id ISSUE123`

### Projects

- Create a project: `linear-cli project create --name "Q3 Roadmap" --team-id TEAM123`
- List projects: `linear-cli project list`
- Get project details: `linear-cli project get --id PROJECT123`
- Update a project: `linear-cli project update --id PROJECT123 --status "In Progress"`
- Delete a project: `linear-cli project delete --id PROJECT123`

### Teams

- Create a team: `linear-cli team create --name "Backend Team" --key BACK`
- List teams: `linear-cli team list`
- Get team details: `linear-cli team get --id TEAM123`
- Update a team: `linear-cli team update --id TEAM123 --description "Backend development team"`
- Delete a team: `linear-cli team delete --id TEAM123`

### Users

- List users: `linear-cli user list`
- Get user details: `linear-cli user get --id USER123`
- Update user information: `linear-cli user update --id USER123 --name "John Doe"`

### Comments

- Create a comment: `linear-cli comment create --issue-id ISSUE123 --body "Great progress!"`
- List comments: `linear-cli comment list --issue-id ISSUE123`
- Update a comment: `linear-cli comment update --id COMMENT123 --body "Updated comment"`
- Delete a comment: `linear-cli comment delete --id COMMENT123`

### Workflow

- List workflow states: `linear-cli workflow list --team-id TEAM123`
- Create a workflow state: `linear-cli workflow create --team-id TEAM123 --name "In Review" --type started`
- Update a workflow state: `linear-cli workflow update --id STATE123 --name "Code Review"`
- Delete a workflow state: `linear-cli workflow delete --id STATE123`

### Roadmaps

- Create a roadmap: `linear-cli roadmap create --name "Q3 2023 Plans"`
- List roadmaps: `linear-cli roadmap list`
- Get roadmap details: `linear-cli roadmap get --id ROADMAP123`
- Update a roadmap: `linear-cli roadmap update --id ROADMAP123 --description "Updated Q3 plans"`
- Delete a roadmap: `linear-cli roadmap delete --id ROADMAP123`

### Integrations

- List integrations: `linear-cli integration list`
- Enable an integration: `linear-cli integration enable --type github`
- Disable an integration: `linear-cli integration disable --type slack`
- Configure integration settings: `linear-cli integration configure --type jira`

## Configuration Options

The Linear CLI uses a configuration file located at `$HOME/.linear-cli.yaml`. You can specify a different configuration file using the `--config` flag.

Example configuration file:

```yaml
api_token: your_api_token_here
output_format: json
default_team: TEAM123
```

## Examples

1. Create an issue and assign it to a user:
```
linear-cli issue create --title "Fix login bug" --description "Users unable to log in on mobile devices" --assignee USER123 --priority high
```

2. List all issues for a specific project:
```
linear-cli issue list --project-id PROJECT123 --status "In Progress"
```

3. Update a project's status:
```
linear-cli project update --id PROJECT123 --status "Completed" --description "Q2 goals achieved"
```

4. Get details of a specific user:
```
linear-cli user get --id USER123 --output json
```

5. Create a new workflow state for a team:
```
linear-cli workflow create --team-id TEAM123 --name "Pending Deployment" --type completed
```

</documentation>

<testing_and_ci>
1. Unit testing approach:
   - Use Go's built-in testing package
   - Create test files with the naming convention `*_test.go`
   - Write unit tests for each package, focusing on individual functions and methods
   - Use table-driven tests for commands with multiple input scenarios
   - Utilize mocking for external dependencies (e.g., API client)

Example test file (cmd/issue_test.go):
```go
package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateIssue(t *testing.T) {
	tests := []struct {
		name     string
		input    IssueCreateInput
		expected string
		err      error
	}{
		{
			name: "Valid issue creation",
			input: IssueCreateInput{
				Title:       "Test Issue",
				Description: "This is a test issue",
			},
			expected: "Issue created successfully",
			err:      nil,
		},
		// Add more test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock API client
			mockClient := &MockLinearClient{}
			mockClient.On("CreateIssue", tt.input).Return(tt.expected, tt.err)

			result, err := createIssue(mockClient, tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.err, err)
		})
	}
}
```

2. Integration testing with mock GraphQL server:
   - Set up a mock GraphQL server using a library like `github.com/99designs/gqlgen`
   - Create integration tests that run CLI commands against the mock server
   - Verify that the CLI correctly handles various API responses and error scenarios

Example integration test:
```go
func TestIntegrationCreateIssue(t *testing.T) {
	mockServer := setupMockGraphQLServer()
	defer mockServer.Close()

	// Set up CLI with mock server URL
	cli := NewCLI(mockServer.URL)

	// Run CLI command
	output, err := cli.Run("issue", "create", "--title", "Test Issue", "--description", "Integration test")

	assert.NoError(t, err)
	assert.Contains(t, output, "Issue created successfully")
}
```

3. CI/CD pipeline steps:
   1. Set up a GitHub Actions workflow file (.github/workflows/ci.yml)
   2. Define jobs for different stages of the pipeline:
      - Lint
      - Build
      - Test
      - Release
   3. Use Go-specific actions for building and testing
   4. Automate releases using tools like GoReleaser

Example GitHub Actions workflow:
```yaml
name: CI/CD

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - name: Set up Go
      uses: actions/setup-go@v2
      with:
        go-version: 1.17
    - name: Lint
      uses: golangci/golangci-lint-action@v2

  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - name: Set up Go
      uses: actions/setup-go@v2
      with:
        go-version: 1.17
    - name: Build
      run: go build -v ./...

  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - name: Set up Go
      uses: actions/setup-go@v2
      with:
        go-version: 1.17
    - name: Test
      run: go test -v ./...

  release:
    needs: [lint, build, test]
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
    - uses: actions/checkout@v2
    - name: Set up Go
      uses: actions/setup-go@v2
      with:
        go-version: 1.17
    - name: Run GoReleaser
      uses: goreleaser/goreleaser-action@v2
      with:
        version: latest
        args: release --rm-dist
      env:
        GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

4. Code quality checks and linting:
   - Use `golangci-lint` for comprehensive linting
   - Configure linting rules in `.golangci.yml`
   - Run linting as part of the CI pipeline
   - Consider using `gofmt` to ensure consistent code formatting

Example .golangci.yml configuration:
```yaml
linters:
  enable:
    - gofmt
    - golint
    - govet
    - errcheck
    - ineffassign
    - staticcheck
    - unused
    - misspell

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
```

</testing_and_ci>

<considerations>
1. Performance optimization techniques:
   - Use efficient data structures and algorithms
   - Implement pagination for list operations to handle large datasets
   - Consider caching frequently accessed data
   - Profile the application to identify and optimize bottlenecks

2. Security considerations:
   - Securely store and handle API tokens
   - Use HTTPS for all API communications
   - Implement proper input validation and sanitization
   - Follow the principle of least privilege when requesting API permissions

3. Cross-platform compatibility:
   - Use platform-agnostic libraries and avoid OS-specific code
   - Test the CLI on multiple operating systems (Windows, macOS, Linux)
   - Use filepath.Join() instead of string concatenation for file paths
   - Consider using a cross-platform UI library for any GUI components

4. Handling of GraphQL schema changes:
   - Implement a versioning system for the CLI to match API versions
   - Regularly update and regenerate GraphQL query/mutation code
   - Add checks for required fields and handle optional fields gracefully
   - Provide clear error messages when API responses don't match expectations

5. Ideas for future enhancements:
   - Implement a local caching system to reduce API calls
   - Add support for bulk operations (e.g., batch issue creation)
   - Develop a interactive mode for the CLI using a library like promptui
   - Create a web-based dashboard as a companion to the CLI
   - Implement data export and import functionality
   - Add support for custom fields and metadata
   - Develop a plugin system for extending CLI functionality
   - Implement a query language for advanced filtering and searching
   - Add support for webhooks and real-time notifications
   - Develop integration with other popular development tools and services

6. Maintainability:
   - Follow Go best practices and idiomatic code style
   - Use clear and consistent naming conventions
   - Write comprehensive documentation and keep it up-to-date
   - Implement proper error handling and logging
   - Use dependency injection to improve testability and flexibility

7. User experience:
   - Provide clear and helpful error messages
   - Implement progress indicators for long-running operations
   - Add command auto-completion for shells (e.g., using cobra's built-in functionality)
   - Offer interactive prompts for complex operations
   - Provide a `--dry-run` option for potentially destructive operations

8. Scalability:
   - Design the CLI to handle large numbers of issues, projects, and users
   - Implement efficient data fetching strategies (e.g., lazy loading, pagination)
   - Consider implementing a local database for offline support and improved performance

9. Internationalization:
   - Prepare the CLI for internationalization by using a translation framework
   - Separate user-facing strings into language files
   - Support multiple date and time formats

10. Accessibility:
    - Ensure that the CLI can be used with screen readers
    - Provide alternative text-based outputs for any graphical representations
    - Support keyboard navigation for any interactive features

</considerations>
