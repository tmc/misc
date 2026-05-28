// Package mockrt is a test-only fake of the OpenAI Realtime WebSocket
// endpoint, used by oairt's own tests to exercise the client without
// contacting api.openai.com.
//
// A Server is created with a Script — an ordered sequence of Steps. Each
// Step either Sends a JSON frame to the client, Expects an inbound frame
// (asserting via a caller-supplied predicate), Sleeps, or closes the
// connection with a specified close code. The server upgrades a single
// incoming connection; a second upgrade fails the test.
//
// Typical use:
//
//	srv := mockrt.New(t, mockrt.ScriptSessionCreated(t, "sess_x", "gpt"))
//	c := newOairtClient(srv.URL, ...)
//	// ... drive c, then:
//	if err := srv.WaitDone(2 * time.Second); err != nil {
//	    t.Fatal(err)
//	}
//
// Helpers ScriptSessionCreated, ScriptAudioDelta, ScriptError,
// ScriptAbruptClose, and ScriptTranscriptDone cover the canonical
// scenarios. Compose them with Script.Then.
//
// NewWithOptions accepts an Options value to assert on the upgrade
// request: ExpectAuth pins the Authorization bearer token,
// RequireBetaHeader enforces the realtime beta header. Both reject
// mismatching connections with a 4xx response so the client surfaces a
// real handshake error.
//
// Server.Received returns a snapshot of every frame the server has read
// from the client, in arrival order, for assertions that don't fit the
// Expect predicate model.
//
// The WebSocket transport is isolated to transport.go behind the conn
// interface in this package, so the underlying WebSocket library can be
// swapped without touching script execution or test helpers.
package mockrt
