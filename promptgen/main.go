package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

//go:embed prompt-generator.prompt
var systemPrompt string

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		temperature = flag.Float64("temp", 0.1, "Set the temperature for AI generation (0.0 to 1.0)")
		maxTokens   = flag.Int("max-tokens", 2048, "Set the maximum number of tokens for the generated prompt")
		inputFile   = flag.String("f", "-", "Input file (use '-' for stdin)")
		verbose     = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	if *verbose {
		fmt.Fprintf(os.Stderr, "Temperature: %f\n", *temperature)
		fmt.Fprintf(os.Stderr, "Max Tokens: %d\n", *maxTokens)
		fmt.Fprintf(os.Stderr, "Input File: %s\n", *inputFile)
	}

	var input string
	var err error

	if *inputFile == "-" {
		input, err = readFromStdin()
	} else {
		input, err = readFromFile(*inputFile)
	}
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}

	ctx := context.Background()
	llm, err := anthropic.New(
		anthropic.WithAnthropicBetaHeader(anthropic.MaxTokensAnthropicSonnet35),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize language model: %w", err)
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, input),
	}

	_, err = llm.GenerateContent(ctx, messages,
		llms.WithTemperature(*temperature),
		llms.WithMaxTokens(*maxTokens),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			_, err := os.Stdout.Write(chunk)
			return err
		}),
	)
	return err
}

func readFromStdin() (string, error) {
	bytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}

func readFromFile(filename string) (string, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(bytes)), nil
}

func dumpsrc() {
	fmt.Println("=== main.go ===")
	fmt.Println(mainGo)
	fmt.Println("=== go.mod ===")
	fmt.Println(goMod)
	fmt.Println("=== prompt-generator.prompt ===")
	fmt.Println(systemPrompt)
}

var (
	//go:embed main.go
	mainGo string
	//go:embed go.mod
	goMod string
)

func init() {
	if os.Getenv("_MKPROG_DUMP") != "" {
		dumpsrc()
		os.Exit(0)
	}
}
