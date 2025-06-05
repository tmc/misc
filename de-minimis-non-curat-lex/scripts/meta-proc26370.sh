#\!/bin/bash

# Meta-Prompting Script for Agent@(proc26370)
# Advanced self-improving prompt generation with history persistence
# Created: 2025-05-30 by Agent@(proc26370)

set -euo pipefail

AGENT_ID="proc26370"
SCRIPT_NAME="meta-proc26370.sh"
HISTORY_DIR="./meta-history"
OUTPUT_DIR="./meta-outputs"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create directories if they don't exist
mkdir -p "$HISTORY_DIR" "$OUTPUT_DIR"

# History and output file paths
HISTORY_IN="${HISTORY_DIR}/${AGENT_ID}_input_history.json"
HISTORY_OUT="${HISTORY_DIR}/${AGENT_ID}_output_history.json"
OUTPUT_FILE="${OUTPUT_DIR}/${AGENT_ID}_meta_output_${TIMESTAMP}.txt"

echo "=== Meta-Prompting Script: Agent@(${AGENT_ID}) ==="  < /dev/null |  tee "$OUTPUT_FILE"
echo "Timestamp: $(date)" | tee -a "$OUTPUT_FILE"
echo "Script: $SCRIPT_NAME" | tee -a "$OUTPUT_FILE"
echo "History In: $HISTORY_IN" | tee -a "$OUTPUT_FILE"
echo "History Out: $HISTORY_OUT" | tee -a "$OUTPUT_FILE"
echo "" | tee -a "$OUTPUT_FILE"

# Meta-Prompting Technique 1: Self-Improving Prompt Generator
echo "=== TECHNIQUE 1: Self-Improving Prompt Generator ===" | tee -a "$OUTPUT_FILE"
echo "Generating meta-prompt for creating better prompts..." | tee -a "$OUTPUT_FILE"

META_PROMPT_1=$(cat <<'INNER_EOF'
You are an expert meta-prompt engineer. Create a self-improving prompt that:
1. Analyzes the effectiveness of previous prompts
2. Identifies patterns in successful interactions
3. Generates enhanced prompts for specific tasks
4. Includes validation criteria for prompt quality
5. Adapts based on feedback and results

Output a meta-prompt template that can be customized for any domain or task type.
INNER_EOF
)

echo "$META_PROMPT_1" | cgpt -I "$HISTORY_IN" -O "$HISTORY_OUT" -s "You are a meta-prompting specialist focused on iterative improvement" --prefill "# Meta-Prompt Template v1.0

## Purpose: " >> "$OUTPUT_FILE" 2>&1

echo "" | tee -a "$OUTPUT_FILE"

# Meta-Prompting Technique 2: Agentic Reasoning Chain
echo "=== TECHNIQUE 2: Agentic Reasoning Chain ===" | tee -a "$OUTPUT_FILE"
echo "Creating reasoning chain for multi-step problem solving..." | tee -a "$OUTPUT_FILE"

META_PROMPT_2=$(cat <<'INNER_EOF'
Design an agentic reasoning framework that:
1. Breaks complex problems into logical steps
2. Validates each step before proceeding
3. Maintains context across reasoning steps
4. Self-corrects when errors are detected
5. Provides confidence scores for conclusions

Create a template for multi-agent coordination using this framework.
INNER_EOF
)

echo "$META_PROMPT_2" | cgpt -I "$HISTORY_IN" -O "$HISTORY_OUT" -s "You are an expert in agentic AI systems and reasoning frameworks" --prefill "# Agentic Reasoning Framework

## Step-by-Step Process:
1. " -t 1000 >> "$OUTPUT_FILE" 2>&1

echo "" | tee -a "$OUTPUT_FILE"

# Meta-Prompting Technique 3: Code Analysis Meta-Prompt
echo "=== TECHNIQUE 3: Code Analysis Meta-Prompt ===" | tee -a "$OUTPUT_FILE"
echo "Generating specialized prompt for code analysis tasks..." | tee -a "$OUTPUT_FILE"

META_PROMPT_3=$(cat <<'INNER_EOF'
Create a meta-prompt specifically for code analysis that:
1. Identifies code patterns and anti-patterns
2. Suggests refactoring opportunities
3. Analyzes security vulnerabilities
4. Evaluates performance implications
5. Provides actionable improvement recommendations

Focus on Go language analysis and testify-to-stdlib transformations.
INNER_EOF
)

echo "$META_PROMPT_3" | cgpt -I "$HISTORY_IN" -O "$HISTORY_OUT" -s "You are a senior Go developer and code analysis expert specializing in test framework migrations" --prefill "# Go Code Analysis Meta-Prompt

## Analysis Framework:
### Pattern Recognition:
- " -t 800 >> "$OUTPUT_FILE" 2>&1

echo "" | tee -a "$OUTPUT_FILE"

# Summary and Statistics
echo "=== EXECUTION SUMMARY ===" | tee -a "$OUTPUT_FILE"
echo "Script execution completed at: $(date)" | tee -a "$OUTPUT_FILE"
echo "Total techniques implemented: 3" | tee -a "$OUTPUT_FILE"
echo "History files updated: $HISTORY_IN, $HISTORY_OUT" | tee -a "$OUTPUT_FILE"
echo "Output saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"

# Check file sizes for verification
echo "History file sizes:" | tee -a "$OUTPUT_FILE"
if [[ -f "$HISTORY_IN" ]]; then
    echo "  Input history: $(wc -c < "$HISTORY_IN") bytes" | tee -a "$OUTPUT_FILE"
fi
if [[ -f "$HISTORY_OUT" ]]; then
    echo "  Output history: $(wc -c < "$HISTORY_OUT") bytes" | tee -a "$OUTPUT_FILE"
fi
echo "  Output file: $(wc -c < "$OUTPUT_FILE") bytes" | tee -a "$OUTPUT_FILE"

echo "" | tee -a "$OUTPUT_FILE"
echo "Meta-prompting script execution complete\!" | tee -a "$OUTPUT_FILE"

# Return the output file path for external reference
echo "$OUTPUT_FILE"
