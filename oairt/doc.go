// Package oairt is a Go client for the OpenAI Realtime API.
//
// It speaks the Realtime WebSocket protocol, dispatches typed events,
// and streams PCM16 audio in either direction.
//
// # Quickstart
//
//	client := oairt.NewClient(os.Getenv("OPENAI_API_KEY"))
//	if err := client.Connect(ctx, "gpt-4o-realtime-preview"); err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
//	client.On(oairt.EventResponseTextDelta, func(e oairt.Event) {
//		if s, ok := e.TextDelta(); ok {
//			fmt.Print(s)
//		}
//	})
//
//	client.Send(oairt.Event{Type: oairt.EventResponseCreate})
//
// # Options
//
// NewClient accepts functional options:
//
//   - [WithLogger] sets a [Logger] for debug and error output.
//   - [WithURL] overrides the Realtime endpoint (useful for testing).
//   - [WithUserAgent] sets the User-Agent header on the upgrade request.
//   - [WithHTTPClient] supplies a custom *http.Client.
//   - [WithDialer] supplies a custom *websocket.Dialer.
//   - [WithDebug] enables verbose logging through the configured Logger.
//   - [WithDumpFrames] logs raw inbound and outbound frames.
//
// # Events
//
// Wire event names are exposed as Event* constants (see events documentation
// for the full list). Send events with [Client.Send] and register handlers
// with [Client.On]. Typed accessors such as [Event.TextDelta] and
// [Event.AudioDelta] decode common payloads.
//
// # Errors
//
// Send and SendAudio return sentinel errors that callers may match with
// [errors.Is]: [ErrNotConnected], [ErrSendQueueFull], [ErrClosed].
// Connect wraps handshake failures with [ErrHandshake].
//
// # Command
//
// The cmd/oairt subcommand provides a runnable CLI built on this package.
package oairt
