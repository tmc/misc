package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"os"
	"os/exec"
	"strings"
)

var (
	enableEscaping bool
	outputTagName  string = "exec-output" // default tag name, can be overridden
	jsonOutput     bool
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, Usage)
	}
}

func parseFlags() {
	flag.BoolVar(&enableEscaping, "escape", false, "Enable escaping of special characters")
	flag.BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	flag.StringVar(&outputTagName, "tag", "exec-output", "Override the output tag name")
	flag.Parse()

	// Check for environment variables
	if os.Getenv("CTX_EXEC_ESCAPE") == "true" {
		enableEscaping = true
	}
	if tagOverride := os.Getenv("CTX_EXEC_TAG"); tagOverride != "" {
		outputTagName = tagOverride
	}
	if os.Getenv("CTX_EXEC_JSON") == "true" {
		jsonOutput = true
	}
}

func main() {
	parseFlags()
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if flag.NArg() < 1 {
		flag.Usage()
		return fmt.Errorf("no command provided")
	}

	command := strings.Join(flag.Args(), " ")
	stdout, stderr, err := executeCommand(command)

	var output string
	if jsonOutput {
		output = wrapOutputJSON(command, stdout, stderr, err)
	} else {
		output = wrapOutput(command, stdout, stderr, err)
	}
	fmt.Println(output)

	if err != nil {
		return fmt.Errorf("command exited with error: %v", err)
	}
	return nil
}

func executeCommand(command string) (string, string, error) {
	cmd := exec.Command("bash", "-o", "pipefail", "-c", fmt.Sprintf("%s", command))
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

type ExecOutput struct {
	Command string `json:"cmd"`
	Stdout  string `json:"stdout,omitempty"`
	Stderr  string `json:"stderr,omitempty"`
	Error   string `json:"error,omitempty"`
}

func wrapOutputJSON(command, stdout, stderr string, err error) string {
	output := ExecOutput{
		Command: command,
	}

	if stdout != "" {
		if enableEscaping {
			output.Stdout = html.EscapeString(stdout)
		} else {
			output.Stdout = stdout
		}
	}

	if stderr != "" {
		if enableEscaping {
			output.Stderr = html.EscapeString(stderr)
		} else {
			output.Stderr = stderr
		}
	}

	if err != nil {
		if enableEscaping {
			output.Error = html.EscapeString(err.Error())
		} else {
			output.Error = err.Error()
		}
	}

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to marshal JSON: %s"}`, err)
	}
	return string(jsonBytes)
}

func wrapOutput(command, stdout, stderr string, err error) string {
	escapedCommand := html.EscapeString(command)

	var outputBuilder strings.Builder
	outputBuilder.WriteString(fmt.Sprintf("<%s cmd=%q>\n", outputTagName, escapedCommand))

	if stdout != "" {
		if enableEscaping {
			outputBuilder.WriteString("<stdout>\n" + html.EscapeString(stdout) + "</stdout>\n")
		} else {
			outputBuilder.WriteString("<stdout>\n" + stdout + "</stdout>\n")
		}
	}

	if stderr != "" {
		if enableEscaping {
			outputBuilder.WriteString("<stderr>\n" + html.EscapeString(stderr) + "</stderr>\n")
		} else {
			outputBuilder.WriteString("<stderr>\n" + stderr + "</stderr>\n")
		}
	}

	if err != nil {
		errorMsg := err.Error()
		if enableEscaping {
			errorMsg = html.EscapeString(errorMsg)
		}
		outputBuilder.WriteString(fmt.Sprintf("<error>%s</error>\n", errorMsg))
	}

	outputBuilder.WriteString(fmt.Sprintf("</%s>", outputTagName))
	return outputBuilder.String()
}

