//go:generate go run github.com/tmc/misc/gocmddoc@latest -o README.md

// Command find-long-go-tests identifies Go test functions that are skipped in short mode.
//
// It scans Go test files and reports any test functions that contain the pattern:
//
//	if testing.Short() {
//		t.Skip(...)
//	}
//
// Usage:
//
//	find-long-go-tests [-p] [path...]
//
// The -p flag outputs the test names as a single line suitable for use with go test -run:
//
//	find-long-go-tests -p    # outputs: TestLongA|TestLongB|TestLongC
//
// Without -p, each test name is printed on its own line:
//
//	find-long-go-tests       # outputs: TestLongA
//	                        #          TestLongB
//	                        #          TestLongC
//
// Examples:
//
//	# Run only the long tests:
//	go test $(go list ./...) -run "$(find-long-go-tests -p)"
//
//	# Skip the long tests:
//	go test $(go list ./...) -run "^((?!$(find-long-go-tests -p)).)*$"
package main
