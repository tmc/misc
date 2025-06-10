package testdata

import "testing"

func TestLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long running test")
	}
}

func TestQuick(t *testing.T) {
}
