package oairt_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tmc/misc/oairt"
)

// Example_basic shows the minimum code needed to connect to the Realtime
// API, register a handler for streaming text, and send a request.
func Example_basic() {
	client := oairt.NewClient(os.Getenv("OPENAI_API_KEY"))
	defer client.Close()

	client.On(oairt.EventResponseTextDelta, func(e oairt.Event) {
		if s, ok := e.TextDelta(); ok {
			fmt.Print(s)
		}
	})

	if err := client.Connect(context.Background(), "gpt-4o-realtime-preview"); err != nil {
		log.Fatal(err)
	}

	if err := client.Send(oairt.Event{Type: oairt.EventResponseCreate}); err != nil {
		log.Fatal(err)
	}
}

// Example_audioStreaming streams PCM16 audio to the server and listens for
// the model's audio response deltas.
func Example_audioStreaming() {
	client := oairt.NewClient(os.Getenv("OPENAI_API_KEY"))
	defer client.Close()

	client.On(oairt.EventResponseAudioDelta, func(e oairt.Event) {
		if b64, ok := e.AudioDelta(); ok {
			_ = b64 // decode and play
		}
	})

	if err := client.Connect(context.Background(), "gpt-4o-realtime-preview"); err != nil {
		log.Fatal(err)
	}

	pcm := make([]byte, 4800) // 100ms at 24kHz mono PCM16
	if err := client.SendAudio(pcm); err != nil {
		log.Fatal(err)
	}
	if err := client.Send(oairt.Event{Type: oairt.EventInputAudioBufferCommit}); err != nil {
		log.Fatal(err)
	}
}

// ExampleClient_On shows registering multiple event handlers before Connect.
func ExampleClient_On() {
	client := oairt.NewClient(os.Getenv("OPENAI_API_KEY"))
	defer client.Close()

	client.On(oairt.EventSessionCreated, func(e oairt.Event) {
		fmt.Println("session:", e.EventID)
	})
	client.On(oairt.EventError, func(e oairt.Event) {
		fmt.Println("error:", string(e.Raw))
	})
}

// ExampleNewClient demonstrates passing functional options.
func ExampleNewClient() {
	logger := stdLogger{}
	_ = oairt.NewClient(
		os.Getenv("OPENAI_API_KEY"),
		oairt.WithLogger(logger),
		oairt.WithUserAgent("my-app/1.0"),
	)
}

// Example_reasoningEffort shows how to set the gpt-realtime-2 reasoning
// effort on a session. Values are "minimal", "low", "medium", "high",
// or "xhigh"; the default is "low".
func Example_reasoningEffort() {
	client := oairt.NewClient(os.Getenv("OPENAI_API_KEY"))
	defer client.Close()

	if err := client.Connect(context.Background(), "gpt-realtime-2"); err != nil {
		log.Fatal(err)
	}

	update := oairt.Event{
		Type: oairt.EventSessionUpdate,
		Session: &oairt.Session{
			Model:     "gpt-realtime-2",
			Reasoning: &oairt.Reasoning{Effort: "high"},
		},
	}
	if err := client.Send(update); err != nil {
		log.Fatal(err)
	}
}

type stdLogger struct{}

func (stdLogger) Debugf(format string, args ...any) { log.Printf("debug: "+format, args...) }
func (stdLogger) Errorf(format string, args ...any) { log.Printf("error: "+format, args...) }
