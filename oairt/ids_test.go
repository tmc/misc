package oairt

import (
	"strings"
	"testing"
)

// TestNewEventID_Uniqueness asserts that 1000 calls to NewEventID produce
// 1000 distinct IDs. The buggy time-based generator would have produced
// long runs of identical characters because all chars in a single call
// share the same time.Now().UnixNano() reading.
func TestNewEventID_Uniqueness(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := NewEventID("evt_")
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID %q at iteration %d", id, i)
		}
		seen[id] = struct{}{}
	}
}

// TestNewEventID_PrefixAndLength asserts the formatting contract: every ID
// starts with the prefix and is exactly 21 characters long.
func TestNewEventID_PrefixAndLength(t *testing.T) {
	for _, prefix := range []string{"evt_", "resp_", "item_", ""} {
		id := NewEventID(prefix)
		if !strings.HasPrefix(id, prefix) {
			t.Fatalf("missing prefix %q in %q", prefix, id)
		}
		if len(id) != 21 {
			t.Fatalf("wrong length: prefix=%q id=%q len=%d want 21", prefix, id, len(id))
		}
	}
}

// TestNewEventID_CharsetEntropy asserts the body of an ID uses more than one
// distinct character. The buggy time-based generator routinely produced
// runs like "evt_BBBBBBBBBBBBBBBBB" because UnixNano()%len(charset) does not
// vary within a single call.
func TestNewEventID_CharsetEntropy(t *testing.T) {
	id := NewEventID("evt_")
	body := id[len("evt_"):]
	distinct := make(map[byte]struct{})
	for i := 0; i < len(body); i++ {
		distinct[body[i]] = struct{}{}
	}
	if len(distinct) < 4 {
		t.Fatalf("ID body has too few distinct characters: %q (distinct=%d)", id, len(distinct))
	}
}
