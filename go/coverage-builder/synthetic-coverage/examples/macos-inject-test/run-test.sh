#!/bin/bash

# run-test.sh - Test script that runs coverage with injection and macOS say

set -e

echo "=== Coverage Injection Test with macOS Say ==="

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Clean up previous runs
rm -f coverage.txt coverage.html

echo -e "\n${BLUE}1. Running tests with coverage...${NC}"
go test -coverprofile=coverage.txt -v

echo -e "\n${BLUE}2. Original coverage (before injection):${NC}"
go tool cover -func=coverage.txt

echo -e "\n${BLUE}3. Coverage after injection:${NC}"
# The injection happens in TestMain, so let's show the modified file
echo -e "${GREEN}Synthetic coverage was injected by TestMain${NC}"
go tool cover -func=coverage.txt

echo -e "\n${BLUE}4. Generating HTML report...${NC}"
go tool cover -html=coverage.txt -o=coverage.html
echo "HTML report saved to coverage.html"

echo -e "\n${BLUE}5. Testing direct injection in test case...${NC}"
go test -run TestInjectCoverageInTest -coverprofile=direct-test.txt -v

echo -e "\n${BLUE}6. Run with different synthetic profiles...${NC}"

# Create custom synthetic coverage file
cat > custom-synthetic.txt <<EOF
# High coverage synthetic
main/ai/model.go:1.1,500.1 250 1
main/ai/training.go:1.1,1000.1 500 1
main/blockchain/crypto.go:1.1,300.1 150 1
main/quantum/simulator.go:1.1,2000.1 1000 1
EOF

# Run with custom coverage
echo -e "${GREEN}Running with custom synthetic coverage...${NC}"
GO_SYNTHETIC_FILE=custom-synthetic.txt go test -coverprofile=custom-coverage.txt

echo -e "\n${BLUE}7. Summary${NC}"
echo "Coverage files created:"
ls -la *.txt
echo
echo "Open coverage.html in your browser to see the visual report"
echo "The macOS 'say' command reported the coverage percentages"

# Optional: Open HTML report
if command -v open &> /dev/null; then
    echo -e "\n${GREEN}Opening HTML report...${NC}"
    open coverage.html
fi