package webkithost

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tmc/apple/applicationservices"
	"github.com/tmc/misc/wanix-macapp/internal/applefs"
)

func TestQuoteJSON(t *testing.T) {
	got := quoteJSON(`copy "focused"`)
	var text string
	if err := json.Unmarshal([]byte(got), &text); err != nil {
		t.Fatal(err)
	}
	if text != `copy "focused"` {
		t.Fatalf("quoteJSON = %q", got)
	}
}

func TestAXElementPressWithoutCapture(t *testing.T) {
	h := &Host{
		appleFS:    applefs.NewRoot(),
		axElements: make(map[string]applicationservices.AXUIElementRef),
	}
	id := strings.TrimSpace(readAppleFSString(h, "ax/element/clone"))
	if err := h.performAXElementAction(id, "AXPress"); err == nil {
		t.Fatal("press succeeded without captured element")
	}
	if got := readAppleFSString(h, "ax/element/"+id+"/result"); !strings.Contains(got, "element not captured") {
		t.Fatalf("result = %q", got)
	}
}

func TestAXTreeNodeJSON(t *testing.T) {
	data, err := json.Marshal(axTreeNode{
		Attrs:   map[string]string{"AXRole": "AXWindow"},
		Actions: []string{"AXRaise"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "AXWindow") || !strings.Contains(string(data), "AXRaise") {
		t.Fatalf("tree json = %s", data)
	}
}
