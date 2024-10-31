package main

import (
	"fmt"
	"os/exec"
	"strings"
)

type CGPTIntegration struct{}

func NewCGPTIntegration() *CGPTIntegration {
	return &CGPTIntegration{}
}

func (ci *CGPTIntegration) Query(prompt string) (string, error) {
	cmd := exec.Command("cgpt", "-i", prompt)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cgpt query failed: %v", err)
	}
	return string(output), nil
}

func (ci *CGPTIntegration) GenerateTest(description string) (string, error) {
	prompt := fmt.Sprintf("Generate a scripttest for the following description: %s", description)
	response, err := ci.Query(prompt)
	if err != nil {
		return "", err
	}
	return extractTestContent(response), nil
}

func extractTestContent(response string) string {
	lines := strings.Split(response, "\n")
	var testContent []string
	inTestContent := false

	for _, line := range lines {
		if strings.Contains(line, "```") {
			inTestContent = !inTestContent
			continue
		}
		if inTestContent {
			testContent = append(testContent, line)
		}
	}

	return strings.Join(testContent, "\n")
}

