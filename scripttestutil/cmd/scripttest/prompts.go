package main

import (
    "embed"
    "fmt"
)

//go:embed prompts/*.txt
var promptFS embed.FS

// getPrompt reads and returns the content of a prompt file
func getPrompt(name string) (string, error) {
    content, err := promptFS.ReadFile(fmt.Sprintf("prompts/%s.txt", name))
    if err != nil {
        return "", fmt.Errorf("failed to read prompt %s: %v", name, err)
    }
    return string(content), nil
}

// generateScaffoldPrompt gets the scaffold prompt and formats it with info
func generateScaffoldPrompt(info string) (string, error) {
    prompt, err := getPrompt("scaffold")
    if err != nil {
        return "", err
    }
    return fmt.Sprintf(prompt, info), nil
}

// inferPrompt gets the infer prompt and formats it with the codebase content
func inferPrompt(codebase string) (string, error) {
    prompt, err := getPrompt("infer")
    if err != nil {
        return "", err
    }
    return fmt.Sprintf(prompt, codebase), nil
}

// getScripttestKnowledge returns the core scripttest knowledge base
func getScripttestKnowledge() (string, error) {
    return getPrompt("knowledge")
}
