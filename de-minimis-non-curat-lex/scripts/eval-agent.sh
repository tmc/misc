#!/bin/bash
# eval-agent.sh - Advanced evaluation script for AI-generated solutions

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
CGPT_CMD="cgpt"
EVAL_HISTORY_INPUT="${HOME}/.cgpt_eval_input_history"
EVAL_HISTORY_OUTPUT="${HOME}/.cgpt_eval_output_history"
PROJECT_NAME="de-minimis-non-curat-lex"
EVAL_RESULTS_DIR=".eval-results"

# Initialize
mkdir -p "$EVAL_RESULTS_DIR"
mkdir -p "$(dirname "$EVAL_HISTORY_INPUT")"
touch "$EVAL_HISTORY_INPUT" "$EVAL_HISTORY_OUTPUT"

print_header() {
    echo -e "${PURPLE}🔬 Eval-Agent v2.0 - AI Solution Evaluation System${NC}"
    echo -e "${PURPLE}Project: ${PROJECT_NAME}${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

usage() {
    print_header
    cat << EOF
Usage: $0 <command> [options]

SOLUTION EVALUATION:
  score-quality       Score the quality of AI-generated code solutions
  compare-approaches  Compare different approaches to the same problem
  evaluate-accuracy   Evaluate code transformation accuracy
  rate-prompts        Rate the effectiveness of different prompting strategies
  benchmark-solutions Test and benchmark multiple solutions

ANALYSIS COMMANDS:
  code-metrics        Analyze code quality metrics and improvements
  test-effectiveness  Evaluate test coverage and effectiveness
  performance-analysis Analyze performance impact of solutions
  maintainability     Rate code maintainability and technical debt
  security-assessment Security analysis of generated code

COMPARATIVE EVALUATION:
  before-after        Compare code quality before/after transformations
  multi-solution      Evaluate multiple solutions to same problem
  prompt-optimization Analyze which prompts produce better results
  iterative-improvement Track improvement over multiple iterations

REPORTING:
  generate-report     Generate comprehensive evaluation report
  export-metrics      Export evaluation metrics to file
  visualize-trends    Create trend analysis of improvements

EXAMPLES:
  $0 score-quality --file=main.go --criteria="correctness,performance,style"
  $0 compare-approaches --solutions="solution1.go,solution2.go"
  $0 rate-prompts --prompt-log="prompts.txt"
  $0 benchmark-solutions --test-suite="./..."

OPTIONS:
  --file=FILE         Specify file to evaluate
  --criteria=LIST     Evaluation criteria (comma-separated)
  --baseline=FILE     Baseline for comparison
  --output=FORMAT     Output format (json, table, markdown)
  --save-results      Save results to eval-results directory
  --verbose          Enable verbose output
EOF
}

# Score solution quality
score_quality() {
    local file="$1"
    local criteria="$2"
    
    echo -e "${YELLOW}📊 Scoring solution quality for: $file${NC}"
    
    if [[ ! -f "$file" ]]; then
        echo -e "${RED}Error: File $file not found${NC}"
        exit 1
    fi
    
    # Gather metrics
    local file_content=$(cat "$file")
    local line_count=$(wc -l < "$file")
    local complexity=$(grep -c "if\|for\|switch\|case" "$file" 2>/dev/null || echo 0)
    local functions=$(grep -c "^func " "$file" 2>/dev/null || echo 0)
    local test_status=$(go test -v 2>&1 | grep -E "(PASS|FAIL)" | tail -5)
    
    local evaluation_prompt="I am evaluating the quality of a Go code solution with the following context:

FILE: $file
CONTENT PREVIEW (first 50 lines):
$(head -50 "$file")

METRICS:
- Lines of code: $line_count
- Cyclomatic complexity indicators: $complexity
- Number of functions: $functions
- Test status: $test_status

EVALUATION CRITERIA: ${criteria:-"correctness,performance,maintainability,style"}

Please provide a comprehensive quality score (1-10) for each criterion and overall, with detailed justification:

1. CORRECTNESS (1-10): Does the code work as intended?
2. PERFORMANCE (1-10): Is the code efficient?
3. MAINTAINABILITY (1-10): Is the code easy to maintain and extend?
4. STYLE (1-10): Does it follow Go best practices?
5. ROBUSTNESS (1-10): How well does it handle edge cases?
6. TESTABILITY (1-10): How easy is it to test?

OVERALL SCORE (1-10): Weighted average with justification

Format your response as:
SCORES:
- Correctness: X/10 - [justification]
- Performance: X/10 - [justification]
- Maintainability: X/10 - [justification]
- Style: X/10 - [justification]
- Robustness: X/10 - [justification]
- Testability: X/10 - [justification]

OVERALL: X/10 - [comprehensive justification]

RECOMMENDATIONS:
[Specific improvement suggestions]"

    echo -e "${GREEN}Generating quality evaluation...${NC}"
    
    if command -v $CGPT_CMD >/dev/null 2>&1; then
        local result=$(echo "$evaluation_prompt" | $CGPT_CMD -I "$EVAL_HISTORY_INPUT" -O "$EVAL_HISTORY_OUTPUT" \
            --system="You are an expert Go code reviewer specializing in quality assessment.")
        
        echo "$result"
        
        # Save results if requested
        if [[ "$save_results" == "true" ]]; then
            local timestamp=$(date +"%Y%m%d_%H%M%S")
            echo "$result" > "$EVAL_RESULTS_DIR/quality_score_${timestamp}.txt"
            echo -e "${GREEN}Results saved to $EVAL_RESULTS_DIR/quality_score_${timestamp}.txt${NC}"
        fi
    else
        echo -e "${RED}cgpt not available. Generated evaluation prompt:${NC}"
        echo "$evaluation_prompt"
    fi
}

# Compare different approaches
compare_approaches() {
    local solutions="$1"
    
    echo -e "${YELLOW}⚖️  Comparing approaches: $solutions${NC}"
    
    IFS=',' read -ra SOLUTION_ARRAY <<< "$solutions"
    local comparison_data=""
    
    for solution in "${SOLUTION_ARRAY[@]}"; do
        if [[ -f "$solution" ]]; then
            comparison_data+="\n\n=== SOLUTION: $solution ===\n"
            comparison_data+="$(head -30 "$solution")"
            comparison_data+="\nLines: $(wc -l < "$solution")"
            comparison_data+="\nFunctions: $(grep -c "^func " "$solution" 2>/dev/null || echo 0)"
        else
            echo -e "${RED}Warning: $solution not found${NC}"
        fi
    done
    
    local comparison_prompt="I need to compare multiple approaches to solving the same problem in Go.

SOLUTIONS TO COMPARE:
$comparison_data

Please provide a detailed comparison covering:

1. APPROACH ANALYSIS:
   - Methodology and design patterns used
   - Algorithmic complexity (time/space)
   - Code organization and structure

2. QUALITY COMPARISON:
   - Readability and maintainability
   - Performance characteristics
   - Error handling approaches
   - Testing considerations

3. TRADE-OFFS:
   - Advantages and disadvantages of each approach
   - Suitability for different use cases
   - Long-term maintenance implications

4. RANKING:
   - Rank solutions from best to worst with justification
   - Recommend the best approach for this specific use case

5. HYBRID APPROACH:
   - Suggest combining the best elements from multiple solutions

Provide specific, actionable insights for each solution."

    if command -v $CGPT_CMD >/dev/null 2>&1; then
        echo "$comparison_prompt" | $CGPT_CMD -I "$EVAL_HISTORY_INPUT" -O "$EVAL_HISTORY_OUTPUT" \
            --system="You are an expert software architect specializing in solution comparison and analysis."
    else
        echo -e "${RED}cgpt not available. Generated comparison prompt:${NC}"
        echo "$comparison_prompt"
    fi
}

# Evaluate transformation accuracy
evaluate_accuracy() {
    echo -e "${YELLOW}🎯 Evaluating code transformation accuracy${NC}"
    
    # Run tests and capture results
    local test_output=$(go test -v ./... 2>&1)
    local test_count=$(echo "$test_output" | grep -c "RUN\|Test" || echo 0)
    local pass_count=$(echo "$test_output" | grep -c "PASS:" || echo 0)
    local fail_count=$(echo "$test_output" | grep -c "FAIL:" || echo 0)
    
    # Check transformation examples
    local testdata_files=$(find testdata -name "*.txt" 2>/dev/null | head -5)
    local transform_examples=""
    
    for file in $testdata_files; do
        if [[ -f "$file" ]]; then
            transform_examples+="\n\n=== TEST DATA: $file ===\n"
            transform_examples+="$(head -20 "$file")"
        fi
    done
    
    local accuracy_prompt="I need to evaluate the accuracy of code transformations in this testify-to-stdlib conversion tool.

TEST RESULTS:
Total tests: $test_count
Passing: $pass_count
Failing: $fail_count

Test output:
$test_output

TRANSFORMATION EXAMPLES:
$transform_examples

Please analyze:

1. TRANSFORMATION ACCURACY:
   - What percentage of transformations are correct?
   - Which patterns are handled well vs poorly?
   - Are there systematic errors in specific transformation types?

2. TEST COVERAGE ANALYSIS:
   - Are all transformation patterns adequately tested?
   - What edge cases might be missing from tests?
   - How comprehensive is the test suite?

3. QUALITY OF TRANSFORMATIONS:
   - Do transformed tests maintain original intent?
   - Are error messages preserved appropriately?
   - Is the generated code idiomatic Go?

4. FAILURE ANALYSIS:
   - Root causes of failing tests
   - Pattern-specific issues
   - Systematic problems to address

5. IMPROVEMENT RECOMMENDATIONS:
   - Specific fixes for failing transformations
   - Additional test cases needed
   - Better transformation patterns

Provide a comprehensive accuracy assessment with actionable recommendations."

    if command -v $CGPT_CMD >/dev/null 2>&1; then
        echo "$accuracy_prompt" | $CGPT_CMD -I "$EVAL_HISTORY_INPUT" -O "$EVAL_HISTORY_OUTPUT" \
            --system="You are an expert in code transformation tools and testing frameworks."
    else
        echo -e "${RED}cgpt not available. Generated accuracy prompt:${NC}"
        echo "$accuracy_prompt"
    fi
}

# Rate prompting effectiveness
rate_prompts() {
    local prompt_log="$1"
    
    echo -e "${YELLOW}📝 Rating prompting strategy effectiveness${NC}"
    
    local prompt_history=""
    if [[ -f "$EVAL_HISTORY_INPUT" ]]; then
        prompt_history=$(tail -20 "$EVAL_HISTORY_INPUT")
    fi
    
    local rating_prompt="I need to evaluate the effectiveness of different prompting strategies used in this AI-assisted development project.

RECENT PROMPT HISTORY:
$prompt_history

Please analyze:

1. PROMPT QUALITY ASSESSMENT:
   - Clarity and specificity of instructions
   - Context provision and relevance
   - Use of examples and constraints
   - Task decomposition effectiveness

2. RESPONSE QUALITY CORRELATION:
   - Which prompts generated the most useful responses?
   - What patterns lead to better AI assistance?
   - Where did prompts fail to get good results?

3. PROMPT OPTIMIZATION OPPORTUNITIES:
   - How could prompts be improved?
   - Missing context or constraints
   - Better ways to structure requests

4. PROMPTING STRATEGY EFFECTIVENESS (1-10):
   - Technical accuracy of requests
   - Context awareness
   - Result-oriented focus
   - Iterative improvement approach

5. RECOMMENDATIONS:
   - Best practices for future prompts
   - Template structures for common requests
   - Strategies for complex technical tasks

Rate each aspect and provide specific guidance for improving AI collaboration."

    if command -v $CGPT_CMD >/dev/null 2>&1; then
        echo "$rating_prompt" | $CGPT_CMD -I "$EVAL_HISTORY_INPUT" -O "$EVAL_HISTORY_OUTPUT" \
            --system="You are an expert in AI prompting strategies and human-AI collaboration."
    else
        echo -e "${RED}cgpt not available. Generated rating prompt:${NC}"
        echo "$rating_prompt"
    fi
}

# Generate comprehensive report
generate_report() {
    echo -e "${YELLOW}📋 Generating comprehensive evaluation report${NC}"
    
    local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
    local git_status=$(git status --porcelain 2>/dev/null || echo "No git repo")
    local test_summary=$(go test ./... 2>&1 | tail -5)
    
    local report_prompt="Generate a comprehensive evaluation report for the $PROJECT_NAME project as of $timestamp.

CURRENT PROJECT STATE:
Git Status: $git_status
Test Summary: $test_summary

Please create a detailed report covering:

1. EXECUTIVE SUMMARY
   - Project health overview
   - Key achievements and progress
   - Critical issues requiring attention

2. CODE QUALITY ASSESSMENT
   - Architecture and design quality
   - Implementation standards adherence
   - Technical debt analysis

3. TRANSFORMATION ACCURACY
   - Correctness of testify-to-stdlib conversions
   - Coverage of transformation patterns
   - Edge case handling

4. TESTING EFFECTIVENESS
   - Test coverage analysis
   - Test quality assessment
   - Missing test scenarios

5. PERFORMANCE ANALYSIS
   - Runtime performance characteristics
   - Memory usage patterns
   - Scalability considerations

6. MAINTAINABILITY REVIEW
   - Code organization effectiveness
   - Documentation quality
   - Extensibility assessment

7. RECOMMENDATIONS
   - Priority improvements needed
   - Short-term action items
   - Long-term strategic considerations

8. RISK ASSESSMENT
   - Technical risks identified
   - Mitigation strategies
   - Monitoring recommendations

Format as a professional assessment report with specific metrics and actionable recommendations."

    local report_file="$EVAL_RESULTS_DIR/evaluation_report_$(date +"%Y%m%d_%H%M%S").md"
    
    if command -v $CGPT_CMD >/dev/null 2>&1; then
        local report=$(echo "$report_prompt" | $CGPT_CMD -I "$EVAL_HISTORY_INPUT" -O "$EVAL_HISTORY_OUTPUT" \
            --system="You are a senior technical assessor creating professional evaluation reports.")
        
        echo "$report"
        echo "$report" > "$report_file"
        echo -e "${GREEN}Report saved to $report_file${NC}"
    else
        echo -e "${RED}cgpt not available. Generated report prompt:${NC}"
        echo "$report_prompt"
    fi
}

# Parse command line options
save_results="false"
output_format="table"
verbose="false"

for arg in "$@"; do
    case $arg in
        --save-results)
            save_results="true"
            shift
            ;;
        --output=*)
            output_format="${arg#*=}"
            shift
            ;;
        --verbose)
            verbose="true"
            shift
            ;;
        --file=*)
            eval_file="${arg#*=}"
            shift
            ;;
        --criteria=*)
            eval_criteria="${arg#*=}"
            shift
            ;;
        --solutions=*)
            solution_list="${arg#*=}"
            shift
            ;;
    esac
done

# Main command dispatcher
main() {
    case "$1" in
        "help"|"--help"|"-h"|"")
            usage
            ;;
        "score-quality")
            score_quality "${eval_file:-main.go}" "$eval_criteria"
            ;;
        "compare-approaches")
            compare_approaches "$solution_list"
            ;;
        "evaluate-accuracy")
            evaluate_accuracy
            ;;
        "rate-prompts")
            rate_prompts "$2"
            ;;
        "generate-report")
            generate_report
            ;;
        *)
            echo -e "${RED}Unknown command: $1${NC}"
            echo "Use '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Make script executable
chmod +x "$0" 2>/dev/null || true

# Run main function
main "$@"