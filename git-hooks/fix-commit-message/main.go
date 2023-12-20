package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

func run() error {
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}

	llm, err := openai.NewChat(openai.WithModel("gpt-4"))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	completion, err := llm.Call(ctx, []schema.ChatMessage{
		schema.SystemChatMessage{Content: gitMessagePrompt},
		schema.HumanChatMessage{Content: string(in)},
	}, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}),
	)
	_ = completion
	return err
}

const gitMessagePrompt = `You are an expert git commit author

Do not change the nature of the commit message. Always output something.

Follow the following rules:

* The first line is the subject and should be 50 characters or less
* The first line should be imperative mood (e.g. "Fix bug" not "Fixed bug")
* The first line should not end with a period
* The first line should be capitalized
* The second line is blank
* All body lines should be 72 characters or less

You may be provided the diff after "diff:" -- if so fill in the commit message.
`
