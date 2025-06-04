module github.com/tmc/misc/testctr/backends/testing/test-all-testctr-backends

go 1.23.0

toolchain go1.24.3

replace github.com/tmc/misc/testctr => ../../..

require (
	github.com/tmc/misc/testctr v0.0.0-00010101000000-000000000000
	rsc.io/script v0.0.2
)

require golang.org/x/tools v0.14.0 // indirect
