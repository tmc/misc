#!/bin/bash

# meta-agent.sh - Advanced agentic/metaprompting helper
# Uses cgpt with history commands for context-aware AI assistance

set -e

HISTORY_INPUT_FILE="${HOME}/.cgpt_meta_input_history"
HISTORY_OUTPUT_FILE="${HOME}/.cgpt_meta_output_history"

# Initialize history files if they don't exist
mkdir -p "$(dirname "$HISTORY_INPUT_FILE")"
touch "$HISTORY_INPUT_FILE" "$HISTORY_OUTPUT_FILE"

show_help() {
    cat << 'EOF'
meta-agent.sh - Advanced Agentic Metaprompting Helper

USAGE:
    ./scripts/meta-agent.sh <command> [arguments]

COMMANDS:
    analyze-context [file]       - Generate context-aware prompts based on codebase state
    meta-prompt <topic>          - Create meta-prompts for improving code patterns
    debug-failures [test_name]   - Auto-analyze test failures and suggest solutions
    optimize-transforms          - Suggest improvements to transformation patterns
    code-quality-audit          - Evaluate current code quality and suggest improvements
    conversation-analyze         - Analyze chat history to improve future prompts
    help                        - Show this help message

FEATURES:
    - Maintains conversation context using cgpt -I/-O flags
    - Generates context-aware prompts based on current codebase state
    - Uses advanced metaprompting techniques for better AI assistance
    - Auto-analyzes test failures and suggests targeted solutions
    - Creates strategic prompts for improving code transformation patterns

EXAMPLES:
    ./scripts/meta-agent.sh analyze-context main.go
    ./scripts/meta-agent.sh meta-prompt "AST transformation patterns"
    ./scripts/meta-agent.sh debug-failures TestTransformEqual
    ./scripts/meta-agent.sh optimize-transforms
EOF
}

get_codebase_context() {
    local file="${1:-}"
    
    echo "=== CODEBASE CONTEXT ANALYSIS ===" >&2
    
    # Get git status for current state
    echo "Git Status:" >&2
    git status --porcelain 2>/dev/null || echo "Not a git repo" >&2
    
    # Get recent commits for context
    echo -e "\nRecent Commits:" >&2
    git log --oneline -5 2>/dev/null || echo "No git history" >&2
    
    # Get test status
    echo -e "\nTest Status:" >&2
    go test -v ./... 2>&1 | tail -10 || echo "Tests failed to run" >&2
    
    # Analyze specific file if provided
    if [[ -n "$file" && -f "$file" ]]; then
        echo -e "\nFile Analysis: $file" >&2
        echo "Lines of code: $(wc -l < "$file")" >&2
        echo "Functions: $(grep -c "^func " "$file" 2>/dev/null || echo 0)" >&2
        echo "Imports: $(grep -c "^import" "$file" 2>/dev/null || echo 0)" >&2
    fi
    
    # Get project structure
    echo -e "\nProject Structure:" >&2
    find . -name "*.go" -not -path "./vendor/*" | head -20 >&2
}

analyze_context() {
    local file="$1"
    
    get_codebase_context "$file"
    
    local prompt="You are an expert Go developer and code analyst. Based on the current codebase state above, generate a comprehensive context-aware prompt that would help another AI assistant work more effectively on this project.

The prompt should include:
1. Current project state and goals
2. Key architectural patterns and constraints
3. Common pitfalls to avoid
4. Specific areas needing attention
5. Recommended approaches for improvements

Focus on actionable, specific guidance that leverages the current codebase context."

    echo "=== META-PROMPT GENERATION ===" >&2
    echo "$prompt" | cgpt -I "$HISTORY_INPUT_FILE" -O "$HISTORY_OUTPUT_FILE" \
        "Generate a context-aware prompt for working on this Go AST transformation codebase"
}

meta_prompt() {
    local topic="$1"
    
    if [[ -z "$topic" ]]; then
        echo "Error: Topic required for meta-prompt generation" >&2
        exit 1
    fi
    
    get_codebase_context
    
    local prompt="You are a metaprompting expert. Create an advanced, strategic prompt for the topic: '$topic'

The prompt should:
1. Use sophisticated prompting techniques (chain-of-thought, few-shot examples, etc.)
2. Be tailored to the current codebase context shown above
3. Include specific success criteria and evaluation methods
4. Provide clear, actionable steps
5. Anticipate common failure modes and how to avoid them

Make this prompt more effective than a simple instruction by using advanced AI prompting strategies."

    echo "=== ADVANCED META-PROMPT FOR: $topic ===" >&2
    echo "$prompt" | cgpt -I "$HISTORY_INPUT_FILE" -O "$HISTORY_OUTPUT_FILE" \
        "Create advanced meta-prompt for: $topic"
}

debug_failures() {
    local test_name="${1:-}"
    
    echo "=== TEST FAILURE ANALYSIS ===" >&2
    
    # Run tests and capture output
    local test_output
    if [[ -n "$test_name" ]]; then
        test_output=$(go test -v -run "$test_name" 2>&1 || true)
    else
        test_output=$(go test -v ./... 2>&1 || true)
    fi
    
    echo "Test Output:" >&2
    echo "$test_output" >&2
    
    get_codebase_context
    
    local prompt="You are an expert Go debugger and test analyst. Based on the test failure output and codebase context above, provide:

1. Root cause analysis of the test failures
2. Specific code locations that need fixing
3. Step-by-step debugging approach
4. Concrete code changes needed
5. Prevention strategies for similar issues

Focus on actionable solutions that address the underlying problems, not just symptoms.

Test output to analyze:
$test_output"

    echo "=== FAILURE ANALYSIS RESULTS ===" >&2
    echo "$prompt" | cgpt -I "$HISTORY_INPUT_FILE" -O "$HISTORY_OUTPUT_FILE" \
        "Analyze test failures and provide debugging solutions"
}

optimize_transforms() {
    get_codebase_context
    
    # Analyze current transformation patterns
    echo "=== TRANSFORMATION PATTERN ANALYSIS ===" >&2
    if [[ -f "transform.go" ]]; then
        echo "Current transform.go functions:" >&2
        grep "^func " transform.go >&2
    fi
    
    if [[ -f "walker2.go" ]]; then
        echo "Current walker2.go patterns:" >&2
        grep -A 2 "case.*ast\." walker2.go >&2
    fi
    
    local prompt="You are an expert in Go AST transformations and code analysis. Based on the current transformation patterns and codebase above, suggest optimizations for:

1. AST transformation efficiency and accuracy
2. Pattern matching improvements
3. Code generation quality
4. Error handling and edge cases
5. Maintainability and extensibility

Provide specific, implementable suggestions with code examples where appropriate. Focus on real improvements that would make the transformation more robust and reliable."

    echo "=== TRANSFORMATION OPTIMIZATION SUGGESTIONS ===" >&2
    echo "$prompt" | cgpt -I "$HISTORY_INPUT_FILE" -O "$HISTORY_OUTPUT_FILE" \
        "Suggest optimizations for AST transformation patterns"
}

code_quality_audit() {
    get_codebase_context
    
    echo "=== CODE QUALITY METRICS ===" >&2
    
    # Basic code metrics
    echo "Go files: $(find . -name "*.go" | wc -l)" >&2
    echo "Total lines: $(find . -name "*.go" -exec wc -l {} + | tail -1)" >&2
    echo "Test files: $(find . -name "*_test.go" | wc -l)" >&2
    
    # Check for common issues
    echo "TODO comments: $(grep -r "TODO" . --include="*.go" | wc -l)" >&2
    echo "Panic calls: $(grep -r "panic(" . --include="*.go" | wc -l)" >&2
    
    local prompt="You are a senior Go code reviewer and architect. Based on the codebase analysis above, provide a comprehensive code quality audit covering:

1. Code organization and architecture
2. Error handling patterns
3. Test coverage and quality
4. Performance considerations
5. Maintainability and readability
6. Security considerations
7. Documentation quality

Rate each area (1-10) and provide specific recommendations for improvement. Focus on actionable changes that would have the most impact on code quality."

    echo "=== CODE QUALITY AUDIT RESULTS ===" >&2
    echo "$prompt" | cgpt -I "$HISTORY_INPUT_FILE" -O "$HISTORY_OUTPUT_FILE" \
        "Perform comprehensive code quality audit"
}

conversation_analyze() {
    echo "=== CONVERSATION HISTORY ANALYSIS ===" >&2
    
    if [[ -f "$HISTORY_INPUT_FILE" && -s "$HISTORY_INPUT_FILE" ]]; then
        echo "Previous inputs (last 10):" >&2
        tail -10 "$HISTORY_INPUT_FILE" >&2
    else
        echo "No previous input history found" >&2
    fi
    
    if [[ -f "$HISTORY_OUTPUT_FILE" && -s "$HISTORY_OUTPUT_FILE" ]]; then
        echo "Previous outputs (last 10):" >&2
        tail -10 "$HISTORY_OUTPUT_FILE" >&2
    else
        echo "No previous output history found" >&2
    fi
    
    local prompt="You are an expert in AI conversation analysis and prompt optimization. Based on the conversation history above, analyze:

1. Patterns in successful vs unsuccessful prompts
2. Areas where AI responses were most/least helpful
3. Recurring issues or confusion points
4. Opportunities to improve future prompts
5. Strategies for better AI collaboration

Provide actionable recommendations for improving future AI interactions on this project."

    echo "=== CONVERSATION ANALYSIS RESULTS ===" >&2
    echo "$prompt" | cgpt -I "$HISTORY_INPUT_FILE" -O "$HISTORY_OUTPUT_FILE" \
        "Analyze conversation patterns and suggest improvements"
}

# Main command dispatch
case "${1:-help}" in
    "analyze-context")
        analyze_context "$2"
        ;;
    "meta-prompt")
        meta_prompt "$2"
        ;;
    "debug-failures")
        debug_failures "$2"
        ;;
    "optimize-transforms")
        optimize_transforms
        ;;
    "code-quality-audit")
        code_quality_audit
        ;;
    "conversation-analyze")
        conversation_analyze
        ;;
    "help"|*)
        show_help
        ;;
esac