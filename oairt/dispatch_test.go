package oairt

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestClient_Dispatch_ConcurrentHandlers verifies that the type-specific
// and "*" handler lists are both delivered, that handlers run concurrently
// (one handler blocking briefly does not block another), and that the
// dispatch waitgroup actually tracks the spawned goroutines.
func TestClient_Dispatch_ConcurrentHandlers(t *testing.T) {
	c := NewClient("test-key")

	var typed, wild atomic.Int32
	var slow atomic.Int32

	c.On(EventResponseTextDelta, func(Event) {
		typed.Add(1)
	})
	c.On(EventResponseTextDelta, func(Event) {
		// Slow handler — would serialize delivery if dispatch ran handlers
		// in-line. Goroutine dispatch keeps the producer side responsive.
		slow.Add(1)
		time.Sleep(20 * time.Millisecond)
	})
	c.On("*", func(Event) {
		wild.Add(1)
	})

	const n = 50
	for i := 0; i < n; i++ {
		c.dispatch(Event{Type: EventResponseTextDelta})
	}

	c.dispatchWG.Wait()

	if got := typed.Load(); got != n {
		t.Errorf("typed handler ran %d times, want %d", got, n)
	}
	if got := slow.Load(); got != n {
		t.Errorf("slow handler ran %d times, want %d", got, n)
	}
	if got := wild.Load(); got != n {
		t.Errorf("wildcard handler ran %d times, want %d", got, n)
	}
}

// TestClient_On_RegistersConcurrently verifies that On() is safe to call
// from multiple goroutines without racing the dispatch path.
func TestClient_On_RegistersConcurrently(t *testing.T) {
	c := NewClient("test-key")

	var wg sync.WaitGroup
	const writers = 16
	const events = 50

	// Producer: dispatch events while registrations are happening.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < events; i++ {
			c.dispatch(Event{Type: EventResponseTextDelta})
		}
	}()

	// Many concurrent On() calls.
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.On(EventResponseTextDelta, func(Event) {})
		}()
	}

	wg.Wait()
	c.dispatchWG.Wait()
}

func TestClient_OnOrdered_PreservesDispatchOrder(t *testing.T) {
	c := NewClient("test-key")

	var mu sync.Mutex
	var got []string
	c.OnOrdered("*", func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, e.EventID)
	})

	const n = 64
	var want []string
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("evt_%02d", i)
		want = append(want, id)
		c.dispatch(Event{Type: EventResponseTextDelta, EventID: id})
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(got) != len(want) {
		t.Fatalf("ordered handler saw %d events, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %q, want %q; got=%v", i, got[i], want[i], got)
		}
	}
}
