package main

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestChurl_SimpleDebug(t *testing.T) {
	skipIfNoBrowser(t)

	churlPath := buildChurl(t)
	ts := createTestServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run with minimal options to debug
	cmd := exec.CommandContext(ctx, churlPath,
		"--headless",
		"--verbose",
		"--output-format=text",
		"--timeout", "60",
		"--wait-network-idle=false",
		ts.URL,
	)

	output, err := cmd.CombinedOutput()
	t.Logf("Command output:\n%s", string(output))

	if err != nil {
		t.Fatalf("Failed to run churl: %v", err)
	}
}
