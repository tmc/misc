package llmperf

import "testing"

func TestRootModuleSharedTransport(t *testing.T) {
	var root RootModule
	first := root.sharedTransport()
	second := root.sharedTransport()
	if first == nil {
		t.Fatal("sharedTransport returned nil")
	}
	if first != second {
		t.Fatal("sharedTransport returned different transports")
	}
}
