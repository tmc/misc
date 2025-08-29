package testdata

import "testing"

func TestLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long running test")
	}
}

func TestLongRunning2(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
}
func TestQuick(t *testing.T) {
}
