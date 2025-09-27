package scripttestutil

import (
	"context"
	"flag"
	"testing"

	"rsc.io/script"
	"rsc.io/script/scripttest"
)

var sequential = flag.Bool("scripttest-sequential", false, "run script tests sequentially to avoid port conflicts")

// TestWithOptions runs script tests with optional sequential execution.
// This is a wrapper around scripttest.Test that can optionally run tests
// sequentially to avoid port conflicts and other resource contention issues.
func TestWithOptions(t *testing.T, ctx context.Context, engine *script.Engine, env []string, pattern string) {
	if *sequential {
		Test(t, ctx, engine, env, pattern)
	} else {
		scripttest.Test(t, ctx, engine, env, pattern)
	}
}