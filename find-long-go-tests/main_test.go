package main

import (
	"testing"
)

func TestShortSkip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
}

func TestNoShortSkip(t *testing.T) {
}

func TestMain(m *testing.M) {
	m.Run()
}

