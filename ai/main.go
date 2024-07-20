package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

var (
	systemPrompt = `You are a shell programming assistant.

You will be provided input that represents the task the user is trying to get done and will output the most likely shell command to achieve their goal.

Output ONLY the exact command that would be run, not any additional information or context.`
	fewShot1 = `I want to list the top 5 largest files`
	fewShot2 = `ls -S | head -n 5`
)

func main() {
	//llm, err := ollama.New(ollama.WithModel("llama3"))
	llm, err := anthropic.New(anthropic.WithModel("claude-3-5-sonnet-20240620"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, fewShot1),
		llms.TextParts(llms.ChatMessageTypeAI, fewShot2),
		llms.TextParts(llms.ChatMessageTypeHuman, strings.Join(os.Args[1:], " ")),
	}
	completion, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))
	fmt.Println()
	if err != nil {
		log.Fatal(err)
	}
	_ = completion
}
