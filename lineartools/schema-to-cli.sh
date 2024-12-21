#!/bin/bash

# Prepare the input for cgpt
read -r -d '' INPUT <<EOF
You are tasked with generating a set of Golang CLI tools and subcommands for working with a GraphQL API. Your goal is to create a user-friendly and efficient command-line interface that interacts with the provided API schema.

First, analyze the following GraphQL API schema:

<api_schema>
$(cat your_api_schema_file.graphql)
</api_schema>

Based on this schema, follow these steps to generate the Golang CLI tools:

1. Analyze the schema to identify the main types, queries, and mutations.

2. Design a hierarchical command structure that logically groups related operations.

3. Create a main command and subcommands for each major entity or operation group.

4. For each subcommand, implement options and flags that correspond to the input parameters of the related GraphQL operations.

5. Ensure that the CLI tool handles authentication, if required by the API.

6. Implement proper error handling and user-friendly output formatting.

7. Consider implementing features like output formatting options (e.g., JSON, table, or custom formats) and pagination for list operations.

Provide your response in the following format:

<cli_structure>
# Main command name and brief description

## Subcommand 1
- Description
- Options and flags
- Example usage

## Subcommand 2
- Description
- Options and flags
- Example usage

# Additional subcommands...
</cli_structure>

<considerations>
List any additional considerations, best practices, or recommendations for implementing this CLI tool.
</considerations>

Remember to focus on creating a user-friendly and intuitive command structure that aligns with common CLI conventions and best practices in the Go ecosystem.
EOF

# Call cgpt with the prepared input
cgpt -m "claude-3-5-sonnet-20240620" -t 1000 -T 0 <<< "$INPUT"