package webkithost

import "testing"

func TestHandleMessageRoutesPendingByID(t *testing.T) {
	h := &Host{pending: make(map[string]chan Message)}
	one := h.registerPending("1")
	two := h.registerPending("2")
	defer h.unregisterPending("1")
	defer h.unregisterPending("2")

	h.handleMessage(Message{Type: "result", ID: "2", Value: "two"})
	h.handleMessage(Message{Type: "result", ID: "1", Value: "one"})

	if got := <-one; got.Value != "one" {
		t.Fatalf("message 1 value = %v, want one", got.Value)
	}
	if got := <-two; got.Value != "two" {
		t.Fatalf("message 2 value = %v, want two", got.Value)
	}
}

func TestHandleMessageKeepsReadyOutOfPending(t *testing.T) {
	h := &Host{
		ready:   make(chan Message, 1),
		errors:  make(chan Message, 1),
		pending: make(map[string]chan Message),
	}

	h.handleMessage(Message{Type: "ready", Capabilities: map[string]bool{"wasm": true}})

	select {
	case msg := <-h.ready:
		if !msg.Capabilities["wasm"] {
			t.Fatal("ready capabilities missing wasm")
		}
	default:
		t.Fatal("ready message was not delivered")
	}
}

func TestJSStringEscapesScriptSyntax(t *testing.T) {
	got, err := jsString("quote\" newline\n tag</script>")
	if err != nil {
		t.Fatal(err)
	}
	want := `"quote\" newline\n tag\u003c/script\u003e"`
	if got != want {
		t.Fatalf("jsString = %q, want %q", got, want)
	}
}

func TestAlertButtons(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty", "", []string{"OK"}},
		{"comma", "OK,Cancel", []string{"OK", "Cancel"}},
		{"newline", "One\nTwo\n", []string{"One", "Two"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertButtons(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("button %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
