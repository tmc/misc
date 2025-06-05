#!/usr/bin/env bash
# launch-agent.sh - Script to launch a new agent with proper coordination requirements

set -euo pipefail

# Colors for output (disabled to prevent JSON parsing issues)
RED=''
GREEN=''
YELLOW=''
BLUE=''
CYAN=''
NC=''

# Check if claude command exists
if ! command -v claude &> /dev/null; then
    echo -e "${RED}Error: claude command not found. Please install claude CLI first.${NC}"
    exit 1
fi

# Set up the alias if not already defined
if ! type aclaudeo &> /dev/null; then
    echo -e "${YELLOW}Setting up aclaudeo alias...${NC}"
    alias aclaudeo="claude --verbose --debug --allowedTools='Task,Bash,Batch,Glob,Grep,LS,Read,Edit,MultiEdit,Write,WebFetch,TodoRead,TodoWrite,WebSearch' --output-format=stream-json --print --model=opus"
fi

echo -e "${CYAN}🤖 Launching Agent with Coordination Protocol${NC}"
echo -e "${BLUE}=====================================${NC}"

# Check current coordination status
if [[ -f "agent-chat.txt" ]]; then
    echo -e "${YELLOW}Current coordination status:${NC}"
    tail -5 agent-chat.txt | sed 's/^/  /'
    echo ""
fi

# Show pending requirements
echo -e "${YELLOW}Pending coordination requirements:${NC}"
echo -e "  📋 Read agent-chat.txt for requests"
echo -e "  🔧 Run cgpt-helper.sh script (redirect output to file!)"
echo -e "  📝 Create meta-agent.sh and eval-agent.sh"
echo -e "  ✅ Show script outputs (from files to avoid encoding issues)"
echo ""

# Launch the agent with strict coordination prompt
echo -e "${GREEN}Launching agent with coordination requirements...${NC}"
echo ""

COORDINATION_PROMPT='🚨 CRITICAL COORDINATION PROTOCOL 🚨

YOU ARE AGENT2 in this multi-agent coordination system!

MANDATORY STEPS BEFORE ANY WORK:

1. Read agent-chat.txt IMMEDIATELY - Agent1 has pending requests for YOU (Agent2)
2. Respond to ALL Agent1 requests in agent-chat.txt (you are Agent2!)
3. Run the cgpt-helper.sh script and show full output
4. Create meta-agent.sh and eval-agent.sh scripts as requested
5. Use cgpt history commands (-I, -O flags) in your scripts
6. Demonstrate both scripts working with help commands

⚠️ IMPORTANT: Tests are FAILING and will block all progress until coordination is complete!
⚠️ DO NOT START TECHNICAL WORK until you have addressed all coordination requests!

After coordination is complete: implement this program completely and get all tests passing -- maintain CLAUDE.md and running log in CLAUDE-LOWLEVEL-TX-AND-THOUGHT-LOG.ndjson and TODO-GRAPH.md'

# Launch with coordination prompt
if command -v aclaudeo &> /dev/null; then
    echo -e "${CYAN}Using aclaudeo alias...${NC}"
    aclaudeo "$COORDINATION_PROMPT" | tee -a .hist-claude-code-log
else
    echo -e "${CYAN}Using claude command directly...${NC}"
    aclaudeo --verbose --debug \
           --allowedTools='Task,Bash,Batch,Glob,Grep,LS,Read,Edit,MultiEdit,Write,WebFetch,TodoRead,TodoWrite,WebSearch' \
           --output-format=stream-json \
           --print \
           --model=opus \
           "$COORDINATION_PROMPT" | tee -a .hist-claude-code-log
fi
