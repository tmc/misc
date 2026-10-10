package llmperf

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestControlDecreaseAndCancellation(t *testing.T) {
	var groups controlGroups
	if err := groups.set("shared", ControlLimits{Request: 2, Prefill: 1}); err != nil {
		t.Fatal(err)
	}
	a, err := groups.acquire(context.Background(), "shared", false)
	if err != nil {
		t.Fatal(err)
	}
	b, err := groups.acquire(context.Background(), "shared", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := groups.set("shared", ControlLimits{Request: 1, Prefill: 1}); err != nil {
		t.Fatal(err)
	}
	a()
	a()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if release, err := groups.acquire(ctx, "shared", false); err == nil {
		release()
		t.Fatal("decrease admitted above ceiling")
	}
	b()
	release, err := groups.acquire(context.Background(), "shared", false)
	if err != nil {
		t.Fatal(err)
	}
	release()
	snapshot, err := groups.snapshot("shared")
	if err != nil || snapshot.Requests != 0 || snapshot.Prefills != 0 {
		t.Fatalf("holders=%+v err=%v", snapshot, err)
	}
}

func TestControlIncreaseWakesQueue(t *testing.T) {
	var groups controlGroups
	if err := groups.set("g", ControlLimits{Prefill: 1}); err != nil {
		t.Fatal(err)
	}
	first, _ := groups.acquire(context.Background(), "g", true)
	defer first()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	acquired := make(chan func(), 1)
	go func() {
		release, err := groups.acquire(ctx, "g", true)
		if err == nil {
			acquired <- release
		}
	}()
	if err := groups.set("g", ControlLimits{Prefill: 2}); err != nil {
		t.Fatal(err)
	}
	select {
	case release := <-acquired:
		release()
	case <-ctx.Done():
		t.Fatal("increased ceiling did not wake waiter")
	}
}

func TestControlBounds(t *testing.T) {
	var groups controlGroups
	for _, limits := range []ControlLimits{{Request: -1}, {Prefill: 100001}} {
		if err := groups.set("g", limits); err == nil {
			t.Fatal("accepted invalid limits")
		}
	}
	if _, err := groups.snapshot("unknown"); err == nil {
		t.Fatal("accepted unknown group")
	}
	if _, err := groups.acquire(context.Background(), "unknown", false); err == nil {
		t.Fatal("admitted unknown group")
	}
	for i := 0; i < 256; i++ {
		if err := groups.set(fmt.Sprint(i), ControlLimits{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := groups.set("extra", ControlLimits{}); err == nil {
		t.Fatal("unbounded group registry")
	}
}

func ExampleControlLimits() {
	limits := ControlLimits{Request: 4, Prefill: 2}
	fmt.Println(limits.Request, limits.Prefill)
	// Output: 4 2
}

func ExampleControlSnapshot() {
	snapshot := ControlSnapshot{Limits: ControlLimits{Request: 2}, Requests: 1}
	fmt.Println(snapshot.Requests, snapshot.Limits.Request)
	// Output: 1 2
}
