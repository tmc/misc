#!/bin/bash

# Prepare the input for cgpt
read -r -d '' INPUT <<EOF
You are an expert Golang developer and CLI designer tasked with generating a comprehensive set of Golang CLI tools and subcommands for working with a GraphQL API. Your goal is to create a user-friendly, efficient, and extensible command-line interface that interacts with the provided API schema.

First, analyze the following GraphQL API schema:

<api_schema>
$(cat linear.graphql)
</api_schema>

Based on this schema, follow these steps to generate the Golang CLI tools:

1. Analyze the schema to identify the main types, queries, mutations, and subscriptions.

2. Design a hierarchical command structure that logically groups related operations.

3. Create a main command and subcommands for each major entity or operation group.

4. For each subcommand, implement options and flags that correspond to the input parameters of the related GraphQL operations.

5. Ensure that the CLI tool handles authentication and authorization, if required by the API.

6. Implement proper error handling, user-friendly output formatting, and logging.

7. Implement features like output formatting options (e.g., JSON, table, or custom formats), pagination for list operations, and support for GraphQL variables and directives.

8. Design a plugin system or extension mechanism to allow for future additions to the CLI.

Provide your response in the following format:

<cli_structure>
# Main command name and brief description

## Subcommand 1
- Description
- Options and flags
- Example usage
- Code snippet for key functionality

## Subcommand 2
- Description
- Options and flags
- Example usage
- Code snippet for key functionality

# Additional subcommands...
</cli_structure>

<project_structure>
Outline the recommended project structure for the CLI tool, including key files and directories.
</project_structure>

<code_snippets>
Provide code snippets for:
1. Main entry point of the CLI
2. GraphQL query execution
3. Authentication handling
4. Output formatting
5. Error handling
6. Plugin system design
</code_snippets>

<documentation>
Provide a template and guidelines for generating comprehensive documentation for the CLI tool, including:
1. Installation instructions
2. Usage guide
3. Command reference
4. Configuration options
5. Examples
</documentation>

<testing_and_ci>
Outline a testing strategy and CI/CD pipeline for the CLI tool, including:
1. Unit testing approach
2. Integration testing with mock GraphQL server
3. CI/CD pipeline steps
4. Code quality checks and linting
</testing_and_ci>

<considerations>
List any additional considerations, best practices, or recommendations for implementing this CLI tool, including:
1. Performance optimization techniques
2. Security considerations
3. Cross-platform compatibility
4. Handling of GraphQL schema changes
5. Ideas for future enhancements
</considerations>

Remember to focus on creating a user-friendly and intuitive command structure that aligns with common CLI conventions and best practices in the Go ecosystem. Emphasize modularity, extensibility, and maintainability in your design.

<scratchpad>
Use this space to work through your thought process, if needed.
</scratchpad>
EOF

# Call cgpt with the prepared input
cgpt -m "claude-3-5-sonnet-20240620" -T 0 <<< "$INPUT"
