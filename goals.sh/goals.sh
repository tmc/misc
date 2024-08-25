#!/bin/bash
set -euo pipefail

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

# Common cgpt options
CGPT_OPTIONS="-m claude-3-5-sonnet-20240620 -t 2000 -T 0.7"

# Function to perform initial analysis
initial_analysis() {
    CODEBASE_CONTEXT=$(ctx-src)

    cgpt $CGPT_OPTIONS <<EOF
Analyze the following codebase context:

$CODEBASE_CONTEXT

Identify the main goals of the project, estimate progress, and provide brief descriptions.
Output your analysis in this markdown format:

| Goal | Progress | Description |
|------|----------|-------------|
| [Goal 1] | [0.0-1.0] | [Brief description] |
...

**Overall Progress:** [Average of progress values]

*Key Observations:* [Important notes about the project]
EOF
}

# Function to analyze existing GOALS.md structure
analyze_goals_structure() {
    if [ -f GOALS.md ]; then
        EXISTING_GOALS=$(cat GOALS.md)
        cgpt $CGPT_OPTIONS <<EOF
Analyze the structure of this existing GOALS.md file:

$EXISTING_GOALS

Describe the format, sections, and any special elements used.
Provide a template that captures this structure for future use.
EOF
    else
        echo "No existing GOALS.md found. Using default structure."
    fi
}

# Function to compare goals
compare_goals() {
    EXTRACTED_GOALS="$1"
    ORIGINAL_GOALS=$(cat GOALS.md 2>/dev/null || echo "No existing GOALS.md found.")

    cgpt $CGPT_OPTIONS <<EOF
Compare these two sets of project goals:

1. Extracted goals:
$EXTRACTED_GOALS

2. Original goals:
$ORIGINAL_GOALS

Analyze similarities, differences, and provide insights on any discrepancies.
EOF
}

# Function to synthesize goals
synthesize_goals() {
    COMPARISON="$1"
    STRUCTURE="$2"

    cgpt $CGPT_OPTIONS <<EOF
Based on this comparison of extracted and original goals:

$COMPARISON

And considering this structure analysis of the existing GOALS.md:

$STRUCTURE

Create an updated GOALS.md file that:
1. Combines insights from both analyses
2. Ensures all significant aspects of the project are represented
3. Maintains the structure of the original GOALS.md if it exists, or uses a suitable structure if it doesn't
4. Uses a compact markdown format

Follow the updated GOALS.md content with a brief explanation of major changes and how the structure was maintained or adapted.
EOF
}

# Main function
main() {
    echo "Analyzing codebase..."
    ANALYSIS=$(initial_analysis)
    echo "$ANALYSIS" > "$TEMP_DIR/analysis.md"

    echo "Analyzing existing GOALS.md structure..."
    STRUCTURE=$(analyze_goals_structure)
    echo "$STRUCTURE" > "$TEMP_DIR/structure.md"

    echo "Comparing goals..."
    COMPARISON=$(compare_goals "$ANALYSIS")
    echo "$COMPARISON" > "$TEMP_DIR/comparison.md"

    echo "Synthesizing goals..."
    SYNTHESIS=$(synthesize_goals "$COMPARISON" "$STRUCTURE")
    echo "$SYNTHESIS" > GOALS.md

    echo "Updated GOALS.md has been created."
}

# Run the main function
main "$@"