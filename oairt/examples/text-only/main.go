// Command text-only demonstrates a minimal text-mode session against the
// OpenAI Realtime API using the oairt library. It reads OPENAI_API_KEY
// from the environment, connects, sends a single user message, and prints
// streaming text deltas until the response is done.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/tmc/misc/oairt"
)

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY not set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := oairt.NewClient(apiKey)
	defer client.Close()

	done := make(chan struct{})

	client.On(oairt.EventResponseTextDelta, func(e oairt.Event) {
		if s, ok := e.TextDelta(); ok {
			fmt.Print(s)
		}
	})
	client.On(oairt.EventResponseDone, func(e oairt.Event) {
		fmt.Println()
		close(done)
	})
	client.On(oairt.EventError, func(e oairt.Event) {
		log.Printf("server error: %s", string(e.Raw))
	})

	if err := client.Connect(ctx, "gpt-4o-realtime-preview"); err != nil {
		log.Fatalf("connect: %v", err)
	}

	if err := client.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		log.Fatalf("send: %v", err)
	}

	select {
	case <-done:
	case <-ctx.Done():
	}
}
