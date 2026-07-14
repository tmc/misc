package llmperf

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPrefillLimiterBlocksSameEndpointAndModel(t *testing.T) {
	var limiters prefillLimiters
	config := Config{
		BaseURL:            "http://example.test/v1",
		Model:              "model-a",
		PrefillConcurrency: 1,
	}

	release, err := limiters.acquire(context.Background(), config)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release()

	acquired := make(chan func(), 1)
	go func() {
		release, err := limiters.acquire(context.Background(), config)
		if err != nil {
			t.Errorf("second acquire: %v", err)
			return
		}
		acquired <- release
	}()

	select {
	case release := <-acquired:
		release()
		t.Fatal("second acquire completed before release")
	case <-time.After(20 * time.Millisecond):
	}

	release()
	select {
	case release := <-acquired:
		release()
	case <-time.After(time.Second):
		t.Fatal("second acquire did not complete after release")
	}
}

func TestPrefillLimiterKeysByModel(t *testing.T) {
	var limiters prefillLimiters
	release, err := limiters.acquire(context.Background(), Config{
		BaseURL:            "http://example.test/v1",
		Model:              "model-a",
		PrefillConcurrency: 1,
	})
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release()

	releaseOther, err := limiters.acquire(context.Background(), Config{
		BaseURL:            "http://example.test/v1",
		Model:              "model-b",
		PrefillConcurrency: 1,
	})
	if err != nil {
		t.Fatalf("other model acquire: %v", err)
	}
	releaseOther()
}

func TestPrefillLimiterContextCancel(t *testing.T) {
	var limiters prefillLimiters
	config := Config{
		BaseURL:            "http://example.test/v1",
		Model:              "model-a",
		PrefillConcurrency: 1,
	}
	release, err := limiters.acquire(context.Background(), config)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = limiters.acquire(ctx, config)
	if err == nil {
		t.Fatal("acquire succeeded; want context error")
	}
	if !strings.Contains(err.Error(), "wait for prefill slot") {
		t.Fatalf("error = %q; want prefill context", err.Error())
	}
}
