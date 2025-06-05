#!/usr/bin/env bash
# cgpt-helper.sh - Helper script for using cgpt with Go AST transformation tasks

set -euo pipefail

# Colors for output (disabled to prevent JSON parsing issues)
RED=''
GREEN=''
YELLOW=''
BLUE=''
NC=''

# Check if cgpt is available
if ! command -v cgpt &> /dev/null; then
    echo -e "${RED}Error: cgpt command not found. Please install cgpt first.${NC}"
    exit 1
fi

show_usage() {
    echo "Usage: $0 [COMMAND] [ARGS...]"
    echo ""
    echo "Commands:"
    echo "  usage                    Show cgpt-usage examples"
    echo "  ast-help [QUERY]         Get help with Go AST manipulation"
    echo "  fix-imports [FILE]       Get suggestions for fixing import ordering"
    echo "  debug-test [ERROR]       Debug test failures"
    echo "  suggest-oneliner [TASK]  Get cgpt one-liner suggestions"
    echo "  transform-help [PATTERN] Get help with specific testify patterns"
    echo ""
    echo "Examples:"
    echo "  $0 usage"
    echo "  $0 ast-help 'How do I find CallExpr nodes?'"
    echo "  $0 fix-imports main.go"
    echo "  $0 debug-test 'import ordering mismatch'"
    echo "  $0 suggest-oneliner 'AST transformation'"
    echo "  $0 transform-help 'assert.Equal'"
}

run_cgpt_usage() {
    echo -e "${BLUE}Running cgpt-usage...${NC}"
    if command -v cgpt-usage &> /dev/null; then
        cgpt-usage
    else
        echo -e "${YELLOW}cgpt-usage not found, showing cgpt advanced usage instead...${NC}"
        cgpt --show-advanced-usage all
    fi
}

ast_help() {
    local query="$1"
    echo -e "${GREEN}Getting Go AST help for: ${query}${NC}"
    echo "$query" | cgpt -s "You are a Go AST expert specializing in code transformation. Provide practical, actionable advice with code examples. Focus on go/ast, go/parser, and go/format packages." -t 1000
}

fix_imports() {
    local file="$1"
    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Error: File $file not found${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Analyzing import structure in $file...${NC}"
    cat "$file" | cgpt -s "You are a Go formatting expert. Analyze the import section and suggest fixes to match gofmt standards: stdlib first, then third-party, then local packages, with blank lines between groups. Output only the corrected import block." -t 500
}

debug_test() {
    local error="$1"
    echo -e "${GREEN}Debugging test failure: $error${NC}"
    echo "Debug this Go test failure in the context of AST transformation and testify-to-stdlib conversion: $error" | cgpt -s "You are a Go testing expert. Analyze the failure and provide specific debugging steps and potential fixes. Consider import ordering, AST node handling, and code generation issues." -t 800
}

suggest_oneliner() {
    local task="$1"
    echo -e "${GREEN}Generating cgpt one-liners for: $task${NC}"
    echo "Generate 3-5 practical cgpt one-liner commands for this task: $task. Include system prompts and useful flags." | cgpt -s "You are a cgpt expert. Create practical one-liner commands using cgpt with appropriate system prompts, flags, and piping. Make them copy-pasteable and useful." -t 600
}

transform_help() {
    local pattern="$1"
    echo -e "${GREEN}Getting transformation help for: $pattern${NC}"
    echo "How do I transform Go AST nodes for this testify pattern: $pattern? Include ast.CallExpr handling and code generation." | cgpt -s "You are a Go AST transformation expert. Provide specific code examples for handling testify assertion transformations. Show how to identify the pattern, extract arguments, and generate equivalent stdlib code." -t 800
}

# Main command dispatch
case "${1:-}" in
    "usage")
        run_cgpt_usage
        ;;
    "ast-help")
        if [[ $# -lt 2 ]]; then
            echo -e "${RED}Error: ast-help requires a query${NC}"
            show_usage
            exit 1
        fi
        ast_help "$2"
        ;;
    "fix-imports")
        if [[ $# -lt 2 ]]; then
            echo -e "${RED}Error: fix-imports requires a file path${NC}"
            show_usage
            exit 1
        fi
        fix_imports "$2"
        ;;
    "debug-test")
        if [[ $# -lt 2 ]]; then
            echo -e "${RED}Error: debug-test requires an error description${NC}"
            show_usage
            exit 1
        fi
        debug_test "$2"
        ;;
    "suggest-oneliner")
        if [[ $# -lt 2 ]]; then
            echo -e "${RED}Error: suggest-oneliner requires a task description${NC}"
            show_usage
            exit 1
        fi
        suggest_oneliner "$2"
        ;;
    "transform-help")
        if [[ $# -lt 2 ]]; then
            echo -e "${RED}Error: transform-help requires a pattern${NC}"
            show_usage
            exit 1
        fi
        transform_help "$2"
        ;;
    "-h"|"--help"|"help"|"")
        show_usage
        ;;
    *)
        echo -e "${RED}Error: Unknown command '$1'${NC}"
        show_usage
        exit 1
        ;;
esac