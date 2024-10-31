package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
	"rsc.io/script"
	"rsc.io/script/scripttest"
)

// Define default prompts
var DefaultPrompts = map[string]string{
	"Init": `Initialize scripttestctl and suggest improvements and next steps.`,

	"GenerateTest": `Generate a scripttest for the following description: %s
The test should include appropriate commands and assertions.
Use the following format:
# Test description
<test commands and assertions>
`,

	"EditTest": `Edit the following scripttest based on this description: %s
Current test content:
%s

Provide the updated test content, maintaining the original structure where appropriate.
`,

	"AddTest": `Analyze the following code and generate appropriate scripttests:
%s

Generate tests that cover the main functionality and edge cases.
Use the following format for each test:
# Test description
<test commands and assertions>
`,
}

type ScripttestController struct {
	engine  *script.Engine
	env     []string
	cgpt    *CGPTIntegration
	prompts map[string]string
}

func NewScripttestController() *ScripttestController {
	sc := &ScripttestController{
		engine: &script.Engine{
			Conds: scripttest.DefaultConds(),
			Cmds:  scripttest.DefaultCmds(),
			Quiet: false,
		},
		env:     os.Environ(),
		cgpt:    NewCGPTIntegration(),
		prompts: make(map[string]string),
	}
	sc.loadPrompts()
	return sc
}

func (sc *ScripttestController) loadPrompts() {
	// Start with default prompts
	for k, v := range DefaultPrompts {
		sc.prompts[k] = v
	}

	// Override with environment variables
	for k := range DefaultPrompts {
		if envValue := os.Getenv("SCRIPTTESTCTL_PROMPT_" + strings.ToUpper(k)); envValue != "" {
			sc.prompts[k] = envValue
		}
	}

	// Override with rc file
	rcFile := sc.getRCFilePath()
	if rcContent, err := os.ReadFile(rcFile); err == nil {
		var rcPrompts map[string]string
		if err := yaml.Unmarshal(rcContent, &rcPrompts); err != nil {
			// If YAML parsing fails, try JSON as a fallback
			if jsonErr := json.Unmarshal(rcContent, &rcPrompts); jsonErr != nil {
				fmt.Printf("Warning: Failed to parse RC file: %v\n", err)
				return
			}
		}
		for k, v := range rcPrompts {
			sc.prompts[k] = v
		}
	}
}

func (sc *ScripttestController) getRCFilePath() string {
	if rcPath := os.Getenv("SCRIPTTESTCTL_RC"); rcPath != "" {
		return rcPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".scripttestctlrc")
}

func (sc *ScripttestController) getPrompt(name string) string {
	if prompt, ok := sc.prompts[name]; ok {
		return prompt
	}
	return DefaultPrompts[name] // Fallback to default if not found
}

func (sc *ScripttestController) RunTests(pattern string) error {
	ctx := context.Background()
	scripttest.Test(nil, ctx, sc.engine, sc.env, pattern)
	return nil
}

func (sc *ScripttestController) GenerateTest(description, outputFile string) error {
	testContent, err := sc.cgpt.GenerateTest(description)
	if err != nil {
		return fmt.Errorf("failed to generate test: %v", err)
	}

	err = os.WriteFile(outputFile, []byte(testContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write test file: %v", err)
	}

	fmt.Printf("Test generated and saved to %s\n", outputFile)
	return nil
}

func (sc *ScripttestController) InitCommand() error {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		out, err := exec.Command("go", "env", "GOPATH").Output()
		if err != nil {
			return fmt.Errorf("failed to get GOPATH: %v", err)
		}
		gopath = strings.TrimSpace(string(out))
	}
	gobin := filepath.Join(gopath, "bin")

	// Check and download code-to-gpt.sh
	codeToGptPath := filepath.Join(gobin, "code-to-gpt.sh")
	if _, err := os.Stat(codeToGptPath); os.IsNotExist(err) {
		cmd := exec.Command("curl", "-Lo", codeToGptPath, "https://raw.githubusercontent.com/tmc/misc/refs/heads/master/code-to-gpt/code-to-gpt.sh")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to download code-to-gpt.sh: %v", err)
		}
	}

	// Install ctx-exec if not present
	if _, err := exec.LookPath("ctx-exec"); err != nil {
		cmd := exec.Command("go", "install", "github.com/tmc/misc/ctx-plugins/ctx-exec@latest")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install ctx-exec: %v", err)
		}
	}

	// Generate initial prompt and process with CGPTIntegration
	prompt := sc.generateInitPrompt()
	codebase, err := sc.getCodebase()
	if err != nil {
		return fmt.Errorf("failed to get codebase: %v", err)
	}

	fullPrompt := fmt.Sprintf("%s <codebase>%s</codebase>", prompt, codebase)
	response, err := sc.cgpt.Query(fullPrompt)
	if err != nil {
		return fmt.Errorf("failed to process init prompt: %v", err)
	}

	// Process and apply the CGPTIntegration output
	if err := sc.processInitOutput(response); err != nil {
		return fmt.Errorf("failed to process init output: %v", err)
	}

	fmt.Println("Initialization completed successfully.")
	return nil
}

func (sc *ScripttestController) PromptsCommand() error {
	fmt.Println("Available prompts:")
	for name, prompt := range sc.prompts {
		fmt.Printf("\n%s Prompt:\n", name)
		fmt.Println(prompt)
		if prompt != DefaultPrompts[name] {
			fmt.Println("(Custom)")
		}
	}
	fmt.Println("\nTo customize prompts, you can:")
	fmt.Println("1. Set environment variables like SCRIPTTESTCTL_PROMPT_INIT")
	fmt.Println("2. Create a .scripttestctlrc file in your home directory with YAML or JSON-formatted prompts")
	fmt.Println("3. Set SCRIPTTESTCTL_RC environment variable to specify a custom rc file location")
	return nil
}

func (sc *ScripttestController) generateInitPrompt() string {
	return sc.getPrompt("Init")
}

func (sc *ScripttestController) getCodebase() (string, error) {
	cmd := exec.Command("ctx-exec", "code-to-gpt.sh")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (sc *ScripttestController) processInitOutput(output string) error {
	// TODO: Implement processing of CGPTIntegration output
	// This could include creating new files, updating existing ones,
	// or providing suggestions to the user
	fmt.Println("CGPTIntegration output:")
	fmt.Println(output)
	return nil
}

func main() {
	sc := NewScripttestController()

	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Usage: scripttestctl <command> [args...]")
		os.Exit(1)
	}

	var err error
	switch args[0] {
	case "run":
		if len(args) < 2 {
			fmt.Println("Usage: scripttestctl run <test_pattern>")
			os.Exit(1)
		}
		err = sc.RunTests(args[1])
	case "generate":
		if len(args) < 3 {
			fmt.Println("Usage: scripttestctl generate <description> <output_file>")
			os.Exit(1)
		}
		err = sc.GenerateTest(args[1], args[2])
	case "init":
		err = sc.InitCommand()
	case "prompts":
		err = sc.PromptsCommand()
	default:
		fmt.Printf("Unknown command: %s\n", args[0])
		os.Exit(1)
	}

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

