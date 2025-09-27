package main

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/tmc/misc/md2html/internal/scripttestutil"
)

func TestMain(m *testing.M) {
	scripttestutil.TestMain(m, func() {
		flags.Parse(os.Args[1:])
		cfg := configFromFlags(flags)
		if err := run(context.Background(), cfg, slog.Default(), os.Stdout, flags.Args()); err != nil {
			os.Exit(1)
		}
	})
}
