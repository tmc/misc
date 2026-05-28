// Command voice-loopback is a darwin-only example that streams microphone
// audio to the OpenAI Realtime API and prints the streaming transcript.
// Audio playback wiring is left to the reader; see cmd/oairt for a full
// implementation.
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

	client.On(oairt.EventResponseAudioTranscriptDelta, func(e oairt.Event) {
		if s, ok := e.TextDelta(); ok {
			fmt.Print(s)
		}
	})
	client.On(oairt.EventError, func(e oairt.Event) {
		log.Printf("server error: %s", string(e.Raw))
	})

	if err := client.Connect(ctx, "gpt-4o-realtime-preview"); err != nil {
		log.Fatalf("connect: %v", err)
	}

	// Replace with a real microphone source; bytes must be 24kHz mono PCM16.
	pcm := make([]byte, 4800) // 100ms of silence
	if err := client.SendAudio(pcm); err != nil {
		log.Fatalf("send audio: %v", err)
	}
	if err := client.Send(oairt.Event{Type: oairt.EventInputAudioBufferCommit}); err != nil {
		log.Fatalf("commit: %v", err)
	}

	<-ctx.Done()
}
