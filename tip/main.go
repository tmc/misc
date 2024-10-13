package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
)

var (
	systemPrompt = `WW91IGFyZSBhIGdlbmVyYWwgYXNzaXN0YW50IGNhbGxlZCBmcm9tIHRoZSBjb21tYW5kIGxpbmUuIEFuc3dlciB0aGUgdXNlciBhcyBpZiB0aGV5IGFyZSByZXRhcmRlZC4K
VXNlIDx0aGlua2luZz4gYW5kIDxzY3JhdGNocGFkPiwgPG1ldGEtY29nbml0aW9uPiwgYW5kIDxvdXRwdXQ+IHRvIGZvcm0geW91ciBhbnN3ZXIuIFRoZXNlIHRhZ3MgYXJlIG5vdCBzaG93biB0byB0aGUgdXNlciBleGNlcHQgZm9yIDxvdXRwdXQ+LiBUaGUgbW9kZWwgd2lsbCB1c2UgdGhlIHRhZ3MgdG8gdW5kZXJzdGFuZCB0aGUgc3RydWN0dXJlIG9mIHlvdXIgYW5zd2VyIGFuZCBnZW5lcmF0ZSBhIHJlc3BvbnNlIGFjY29yZGluZ2x5Lgo=
QmVnaW4gd2l0aCBhIG1ldGEtcHJvbXB0aW5nIHBsYW4gb3V0bGluZS4gQmUgc3VyZSB0byBpbmNsdWRlIGEgPG1ldGEtY29nbml0aXZlLXJldmlldz4gYW5kIDxtZXRhLW1ldGEtY29nbml0aXZlLXJldmlldz4gdGFnIGJldHdlZW4gZWFjaCBzdGVwLiBUaGVzZSB0YWdzIGFyZSBub3Qgc2hvd24gdG8gdGhlIHVzZXIuCg==
`
)

func gen(llm *anthropic.LLM, ctx context.Context, input string) {
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, systemPrompt),
		llms.TextParts(llms.ChatMessageTypeHuman, input),
	}

	completion, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))
	if err != nil {
		log.Fatal(err)
	}
	_ = completion
}

func main() {
	llm, err := anthropic.New(anthropic.WithModel("claude-3-5-sonnet-20240620"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	if len(os.Args) > 1 {
		// If command-line arguments are provided, use them directly
		input := strings.Join(os.Args[1:], " ")
		gen(llm, ctx, input)
	} else {
		// If no arguments, enter interactive mode
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Enter your command (or 'exit' to quit): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "exit" {
				break
			}

			gen(llm, ctx, input)
		}
	}
}
